package metrics

import (
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"
)

// Event представляет одно событие в журнале процесса.
type Event struct {
    SessionID   string
    Timestamp   time.Time
    Description string
    Result      string
}

// ProcessInstance представляет последовательность событий для одного экземпляра процесса.
type ProcessInstance struct {
    ID     string
    Events []Event
}

// MetricDefinition содержит общее описание метрики (одно на тип метрики).
type MetricDefinition struct {
    Name        string `json:"name"`  // Название метрики
    Category    string `json:"category"`  // Категория (Зацикливание, Длительность, Сложность и т.д.)
    Calculation string `json:"calculation"`  // Как считается метрика
    Impact      string `json:"impact"`  // Что означает и какой эффект
    Threshold   float64 `json:"threshold"` // Пороговое значение
}

// MetricOccurrence представляет одно конкретное проявление метрики.
type MetricOccurrence struct {
	InstanceID          string  // ID экземпляра процесса
	Value               float64 // Значение метрики
	WastedDurationSeconds float64 // Потерянное время в секундах ("финансовый эффект")
	Details             string  // Краткая детализация (опционально)
}

// InefficiencyMetric содержит агрегированный результат по метрике.
type InefficiencyMetric struct {
    Definition  MetricDefinition `json:"definition"`
    Occurrences []MetricOccurrence `json:"occurrences"`
    TotalValue            float64 `json:"total_value"`             // Агрегированное значение
	TotalWastedDuration   float64 `json:"total_wasted_duration"`   // Общее потерянное время в секундах
    Count       int `json:"count"`     // Количество вхождений
    Exceeded    bool `json:"exceeded"`    // Превышен ли порог
}

// DurationMetricsResult содержит агрегированные метрики длительности и их вхождения.
type DurationMetricsResult struct {
	Occurrences         []MetricOccurrence
	Average             float64
	IQR                 float64
	AnomalousCount      int
	StageTrendSlope     float64
	InstanceTrendSlope  float64
}

// ActivityCount представляет количество вхождений активности.
type ActivityCount struct {
	Activity string `json:"activity"`
	Count    int    `json:"count"`
}

// PathCount представляет количество вхождений пути.
type PathCount struct {
	Path  []string `json:"path"`
	Count int      `json:"count"`
}

// MetricsReport содержит результаты анализа процесса.
type MetricsReport struct {
	TotalProcessInstances  int             `json:"total_process_instances"`
	TotalEvents            int             `json:"total_events"`
	AverageProcessDuration float64         `json:"average_process_duration"`
	MedianProcessDuration  float64         `json:"median_process_duration"`
	MostFrequentActivities []ActivityCount `json:"most_frequent_activities"`
	MostFrequentPaths      []PathCount     `json:"most_frequent_paths"`
	// Агрегированные метрики длительности этапов
	StageDurationAverage   float64         `json:"stage_duration_average"`
	StageDurationIQR       float64         `json:"stage_duration_iqr"`
	AnomalousStageCount    int             `json:"anomalous_stage_count"`
	StageDurationTrendSlope float64        `json:"stage_duration_trend_slope"`
	Metrics                []InefficiencyMetric `json:"metrics"`
}

// Analyzer — основной компонент для вычисления метрик.
type Analyzer struct {
    definitions map[string]MetricDefinition
    Logger      *slog.Logger
}

// NewAnalyzer создает новый анализатор с предопределёнными определениями метрик.
func NewAnalyzer() *Analyzer {
    return &Analyzer{
        Logger: slog.Default(),
        definitions: initMetricDefinitions(),
    }
}

// initMetricDefinitions инициализирует справочник определений метрик.
func initMetricDefinitions() map[string]MetricDefinition {
    return map[string]MetricDefinition{
        "Self-Loop": {
            Name:        "Самозацикливание",
            Category:    "Зацикливание",
            Calculation: "Обнаружение повторения одной операции подряд (A→A)",
            Impact:      "Указывает на технические ошибки, переадресацию задач или дублирование в логе. Приводит к росту длительности.",
            Threshold:   0.0,
        },
        "Return to Previous Stage": {
            Name:        "Возврат на предыдущий этап",
            Category:    "Зацикливание",
            Calculation: "Обнаружение возврата к операции через одну (A→B→A)",
            Impact:      "Возврат на доработку. Указывает на переделки или ошибки в процессе.",
            Threshold:   0.0,
        },
        "Ping-Pong": {
            Name:        "Пинг-понг",
            Category:    "Зацикливание",
            Calculation: "Обнаружение повторяющегося чередования двух операций (A→B→A→B)",
            Impact:      "Неэффективное взаимодействие между этапами или ошибки маршрутизации.",
            Threshold:   0.0,
        },
        "Return to Start": {
            Name:        "Возврат к началу",
            Category:    "Зацикливание",
            Calculation: "Обнаружение возврата к первой операции процесса",
            Impact:      "Перезапуск процесса из-за критических ошибок или несоответствия условий.",
            Threshold:   0.0,
        },
        "Rework": {
            Name:        "Переделка",
            Category:    "Зацикливание",
            Calculation: "Подсчёт повторений произвольной операции в экземпляре",
            Impact:      "Необходимость исправления ошибок или доработок, увеличение времени выполнения.",
            Threshold:   0.0,
        },
        "Anomalously Long Stage": {
            Name:        "Аномально долгий этап",
            Category:    "Длительность",
            Calculation: "Выявление выбросов длительности этапов через межквартильный размах (IQR > Q3 + 1.5*IQR)",
            Impact:      "Узкие места или проблемы производительности на конкретных этапах.",
            Threshold:   0.0, // Рассчитывается динамически
        },
        "Increasing Stage Duration Trend": {
            Name:        "Рост длительности этапа",
            Category:    "Длительность",
            Calculation: "Линейная регрессия длительности этапов во времени (положительный наклон)",
            Impact:      "Деградация производительности или рост сложности задач со временем.",
            Threshold:   0.0,
        },
        "Increasing Process Instance Duration Trend": {
            Name:        "Рост длительности экземпляра",
            Category:    "Длительность",
            Calculation: "Линейная регрессия длительности экземпляров во времени",
            Impact:      "Общее ухудшение производительности процесса со временем.",
            Threshold:   0.0,
        },
        "Manual/Unlogged Stage": {
            Name:        "Ручной/незарегистрированный этап",
            Category:    "Логирование",
            Calculation: "Поиск этапов, занимающих >80% времени экземпляра",
            Impact:      "Отсутствие логирования делает невозможной оценку процесса. Потенциальные ручные операции.",
            Threshold:   80.0,
        },
        "High Process Variability": {
            Name:        "Высокая вариативность процесса",
            Category:    "Сложность",
            Calculation: "Отношение уникальных путей к общему числу экземпляров (>80%)",
            Impact:      "Каждый путь уникален — невозможность стандартизации, визуализации и оптимизации.",
            Threshold:   80.0,
        },
        "Low Process Completion Rate": {
            Name:        "Низкий процент завершения",
            Category:    "Завершённость",
            Calculation: "Доля экземпляров, завершивших полный цикл (начало→конец)",
            Impact:      "Процесс прерывается или не доходит до конца, что снижает эффективность.",
            Threshold:   100.0,
        },
        "High Error Rate": {
            Name:        "Высокий процент ошибок",
            Category:    "Качество",
            Calculation: "Сравнение количества ошибок с успешными экземплярами",
            Impact:      "Нестабильность процесса, превышение ошибок над успешными выполнениями.",
            Threshold:   0.0,
        },
    }
}

// Analyze выполняет анализ экземпляров процесса.
func (a *Analyzer) Analyze(instances map[string]*ProcessInstance) *MetricsReport {
	report := &MetricsReport{}

	// 1. Общее количество экземпляров процессов
	report.TotalProcessInstances = len(instances)

	// 2. Общее количество событий
	var totalEvents int
	for _, instance := range instances {
		totalEvents += len(instance.Events)
	}
	report.TotalEvents = totalEvents

	// 3. Средняя и медианная продолжительность процесса
	var processDurations []float64
	for _, instance := range instances {
		if len(instance.Events) > 1 {
			start := instance.Events[0].Timestamp
			end := instance.Events[len(instance.Events)-1].Timestamp
			processDurations = append(processDurations, end.Sub(start).Seconds())
		}
	}

	if len(processDurations) > 0 {
		sort.Float64s(processDurations)
		var sumDuration float64
		for _, d := range processDurations {
			sumDuration += d
		}
		report.AverageProcessDuration = sumDuration / float64(len(processDurations))

		mid := len(processDurations) / 2
		if len(processDurations)%2 == 0 {
			report.MedianProcessDuration = (processDurations[mid-1] + processDurations[mid]) / 2
		} else {
			report.MedianProcessDuration = processDurations[mid]
		}
	} else {
		report.AverageProcessDuration = 0.0
		report.MedianProcessDuration = 0.0
	}

	// 4. Наиболее частые действия
	activityCounts := make(map[string]int)
	for _, instance := range instances {
		for _, event := range instance.Events {
			activityCounts[event.Description]++
		}
	}

	var sortedActivities []ActivityCount
	for activity, count := range activityCounts {
		sortedActivities = append(sortedActivities, ActivityCount{Activity: activity, Count: count})
	}
	sort.Slice(sortedActivities, func(i, j int) bool {
		return sortedActivities[i].Count > sortedActivities[j].Count
	})

	if len(sortedActivities) > 5 {
		report.MostFrequentActivities = sortedActivities[:5]
	} else {
		report.MostFrequentActivities = sortedActivities
	}

	// 5. Наиболее частые пути
	pathCounts := make(map[string]int)
	pathMap := make(map[string][]string)
	for _, instance := range instances {
		if len(instance.Events) > 0 {
			path := make([]string, len(instance.Events))
			for i, event := range instance.Events {
				path[i] = event.Description
			}
			pathKey := fmt.Sprintf("%v", path)
			pathCounts[pathKey]++
			pathMap[pathKey] = path
		}
	}

	var sortedPaths []PathCount
	for pathKey, count := range pathCounts {
		sortedPaths = append(sortedPaths, PathCount{Path: pathMap[pathKey], Count: count})
	}
	sort.Slice(sortedPaths, func(i, j int) bool {
		return sortedPaths[i].Count > sortedPaths[j].Count
	})

	if len(sortedPaths) > 5 {
		report.MostFrequentPaths = sortedPaths[:5]
	} else {
		report.MostFrequentPaths = sortedPaths
	}

	// Собираем все вхождения метрик
	rawMetrics := []struct {
		metricType string
		occurrence MetricOccurrence
	}{}

	// Вызываем функции расчёта метрик
	rawMetrics = append(rawMetrics, a.collectLoopingMetrics(instances)...)
	rawMetrics = append(rawMetrics, a.collectDurationMetrics(instances)...)
	rawMetrics = append(rawMetrics, a.collectManualStageMetrics(instances)...)
	rawMetrics = append(rawMetrics, a.collectComplexityMetrics(instances)...)
	rawMetrics = append(rawMetrics, a.collectCompletionMetrics(instances)...)
	rawMetrics = append(rawMetrics, a.collectErrorMetrics(instances)...)

	// Агрегируем по типам метрик
	aggregated := make(map[string]*InefficiencyMetric)

	// Сначала инициализируем все метрики с нулевыми значениями
	for key, def := range a.definitions {
		aggregated[key] = &InefficiencyMetric{
			Definition:  def,
			Occurrences: []MetricOccurrence{},
			Count:       0,
			Exceeded:    false,
		}
	}

	// Теперь заполняем найденные вхождения
	for _, raw := range rawMetrics {
		if metric, exists := aggregated[raw.metricType]; exists {
			metric.Occurrences = append(metric.Occurrences, raw.occurrence)
			metric.TotalValue += raw.occurrence.Value
			metric.TotalWastedDuration += raw.occurrence.WastedDurationSeconds
			metric.Count++
			if raw.occurrence.Value > metric.Definition.Threshold {
				metric.Exceeded = true // Устанавливаем флаг, если хотя бы одно вхождение превышает порог
			}
		}
	}

	// Преобразуем в слайс
	for _, metric := range aggregated {
		metric.TotalValue = math.Round(metric.TotalValue*10) / 10
		report.Metrics = append(report.Metrics, *metric)
	}

	return report
}

// collectLoopingMetrics собирает вхождения метрик зацикливания.
func (a *Analyzer) collectLoopingMetrics(instances map[string]*ProcessInstance) []struct {
    metricType string
    occurrence MetricOccurrence
} {
    var results []struct {
        metricType string
        occurrence MetricOccurrence
    }

    for _, instance := range instances {
        if len(instance.Events) < 2 {
            continue
        }

        // Self-loop
		for i := 1; i < len(instance.Events); i++ {
			if instance.Events[i].Description == instance.Events[i-1].Description {
				results = append(results, struct {
					metricType string
					occurrence MetricOccurrence
				}{
					metricType: "Self-Loop",
					occurrence: MetricOccurrence{
						InstanceID:          instance.ID,
						Value:               1.0,
						WastedDurationSeconds: instance.Events[i].Timestamp.Sub(instance.Events[i-1].Timestamp).Seconds(),
						Details:             fmt.Sprintf("Шаг %d: '%s'", i, instance.Events[i].Description),
					},
				})
			}
		}

        // Return to Previous Stage
		for i := 2; i < len(instance.Events); i++ {
			if instance.Events[i].Description == instance.Events[i-2].Description {
				results = append(results, struct {
					metricType string
					occurrence MetricOccurrence
				}{
					metricType: "Return to Previous Stage",
					occurrence: MetricOccurrence{
						InstanceID:          instance.ID,
						Value:               1.0,
						WastedDurationSeconds: instance.Events[i].Timestamp.Sub(instance.Events[i-2].Timestamp).Seconds(),
						Details:             fmt.Sprintf("Шаг %d: '%s'", i, instance.Events[i].Description),
					},
				})
			}
		}

        // Ping-pong
        for i := 3; i < len(instance.Events); i++ {
            if instance.Events[i].Description == instance.Events[i-2].Description &&
                instance.Events[i-1].Description == instance.Events[i-3].Description {
                results = append(results, struct {
                    metricType string
                    occurrence MetricOccurrence
                }{
                    metricType: "Ping-Pong",
                    occurrence: MetricOccurrence{
						InstanceID:          instance.ID,
						Value:               1.0,
						WastedDurationSeconds: instance.Events[i-1].Timestamp.Sub(instance.Events[i-3].Timestamp).Seconds(),
						Details:             fmt.Sprintf("Шаг %d: '%s' ↔ '%s'", i, instance.Events[i-1].Description, instance.Events[i].Description),
					},
                })
            }
        }

        // Return to Start
        if len(instance.Events) > 1 {
            firstEvent := instance.Events[0]
            for i := 1; i < len(instance.Events); i++ {
                if instance.Events[i].Description == firstEvent.Description {
                    results = append(results, struct {
                        metricType string
                        occurrence MetricOccurrence
                    }{
                        metricType: "Return to Start",
                        occurrence: MetricOccurrence{
                            InstanceID: instance.ID,
                            Value:      1.0,
                            Details:    fmt.Sprintf("Шаг %d: возврат к '%s'", i, instance.Events[i].Description),
                        },
                    })
                }
            }
        }

        // Rework
		eventIndices := make(map[string][]int)
		for i, event := range instance.Events {
			eventIndices[event.Description] = append(eventIndices[event.Description], i)
		}

		for desc, indices := range eventIndices {
			if len(indices) > 1 {
				var wastedDuration float64
				// Суммируем длительность всех переделанных этапов, кроме последнего
				for i := 0; i < len(indices)-1; i++ {
					currentIndex := indices[i]
					// Убедимся, что следующий эвент существует, чтобы посчитать длительность
					if currentIndex+1 < len(instance.Events) {
						wastedDuration += instance.Events[currentIndex+1].Timestamp.Sub(instance.Events[currentIndex].Timestamp).Seconds()
					}
				}

				results = append(results, struct {
					metricType string
					occurrence MetricOccurrence
				}{
					metricType: "Rework",
					occurrence: MetricOccurrence{
						InstanceID:          instance.ID,
						Value:               float64(len(indices) - 1),
						WastedDurationSeconds: wastedDuration,
						Details:             fmt.Sprintf("Этап '%s' повторён %d раз", desc, len(indices)),
					},
				})
			}
		}
    }

    return results
}

// collectDurationMetrics собирает метрики длительности.
func (a *Analyzer) collectDurationMetrics(instances map[string]*ProcessInstance) []struct {
    metricType string
    occurrence MetricOccurrence
} {
    var results []struct {
        metricType string
        occurrence MetricOccurrence
    }
    var durations []float64

    // Собираем длительности всех операций
    for _, instance := range instances {
        if len(instance.Events) < 2 {
            // Пропускаем экземпляры с менее чем двумя событиями, так как длительность не может быть рассчитана.
            a.Logger.Warn("Экземпляр имеет менее двух событий, длительность не может быть рассчитана", "instance_id", instance.ID)
            continue
        }
        for i := 0; i < len(instance.Events)-1; i++ {
            event1 := instance.Events[i]
            event2 := instance.Events[i+1]

            if event1.Timestamp.IsZero() || event2.Timestamp.IsZero() {
                a.Logger.Warn("Обнаружена нулевая временная метка, пропуск расчета длительности", "instance_id", instance.ID, "event_index_1", i, "event_index_2", i+1)
                continue
            }

            if event2.Timestamp.Before(event1.Timestamp) {
                a.Logger.Warn("Некорректный порядок временных меток", "instance_id", instance.ID, "event_index_1", i, "timestamp_1", event1.Timestamp, "event_index_2", i+1, "timestamp_2", event2.Timestamp)
                continue
            }

            duration := event2.Timestamp.Sub(event1.Timestamp)
            durations = append(durations, duration.Seconds())
        }
    }

    if len(durations) == 0 {
        a.Logger.Warn("Нет доступных длительностей для расчета метрик")
        return results
    }

    if len(durations) < 4 {
        a.Logger.Warn("Недостаточно длительностей для расчета IQR", "count", len(durations))
        // В этом случае мы можем решить, возвращать ли частичные результаты или нет.
        // Пока что, мы просто продолжим, но без расчетов, требующих 4+ длительностей.
    }

    // Вычисляем среднее и стандартное отклонение
    var sum float64
    for _, d := range durations {
        sum += d
    }
    avgDuration := sum / float64(len(durations))

    // Расчет аномалий возможен только при наличии достаточного количества данных
	if len(durations) >= 4 {
		// Сортируем и вычисляем IQR
		sort.Float64s(durations)
		q1Index := int(math.Round(float64(len(durations)-1) * 0.25))
		q3Index := int(math.Round(float64(len(durations)-1) * 0.75))
		q1 := durations[q1Index]
		q3 := durations[q3Index]
		iqr := q3 - q1
		outlierThreshold := q3 + 1.5*iqr

		// Аномально длинные этапы
		for _, instance := range instances {
			for i := 0; i < len(instance.Events)-1; i++ {
				duration := instance.Events[i+1].Timestamp.Sub(instance.Events[i].Timestamp)
				if duration.Seconds() > outlierThreshold {
					results = append(results, struct {
						metricType string
						occurrence MetricOccurrence
					}{
						metricType: "Anomalously Long Stage",
						occurrence: MetricOccurrence{
							InstanceID: instance.ID,
							Value:      duration.Seconds(),
							Details:    fmt.Sprintf("Этап '%s': %.2f сек (avg: %.2f сек)", instance.Events[i].Description, duration.Seconds(), avgDuration),
						},
					})
				}
			}
		}
	}

    // Тренд длительности этапов
    slope, _ := calculateLinearRegression(durations)
	angleRadians := math.Atan(slope)
	angleDegrees := angleRadians * 180 / math.Pi
	if angleDegrees > 5.0 {
        results = append(results, struct {
            metricType string
            occurrence MetricOccurrence
        }{
            metricType: "Increasing Stage Duration Trend",
            occurrence: MetricOccurrence{
                InstanceID: "ALL",
                Value:      slope,
                Details:    fmt.Sprintf("Наклон: %.4f сек/операцию", slope),
            },
        })
    }

    // Тренд длительности экземпляров
    var instanceDurations []float64
    for _, instance := range instances {
        if len(instance.Events) > 1 {
            instanceDurations = append(instanceDurations, instance.Events[len(instance.Events)-1].Timestamp.Sub(instance.Events[0].Timestamp).Seconds())
        }
    }

    if len(instanceDurations) > 1 {
        instanceSlope, _ := calculateLinearRegression(instanceDurations)
        if instanceSlope > 0 {
            results = append(results, struct {
                metricType string
                occurrence MetricOccurrence
            }{
                metricType: "Increasing Process Instance Duration Trend",
                occurrence: MetricOccurrence{
                    InstanceID: "ALL",
                    Value:      instanceSlope,
                    Details:    fmt.Sprintf("Наклон: %.4f сек/экземпляр", instanceSlope),
                },
            })
        }
    }

    return results
}

// collectManualStageMetrics собирает метрики ручных этапов.
func (a *Analyzer) collectManualStageMetrics(instances map[string]*ProcessInstance) []struct {
    metricType string
    occurrence MetricOccurrence
} {
    var results []struct {
        metricType string
        occurrence MetricOccurrence
    }

    for _, instance := range instances {
        if len(instance.Events) < 2 {
            continue
        }

        totalInstanceDuration := instance.Events[len(instance.Events)-1].Timestamp.Sub(instance.Events[0].Timestamp).Seconds()

        for i := 0; i < len(instance.Events)-1; i++ {
            stageDuration := instance.Events[i+1].Timestamp.Sub(instance.Events[i].Timestamp).Seconds()
            percentage := (stageDuration / totalInstanceDuration) * 100
            
            if totalInstanceDuration > 0 && percentage > 80.0 {
                results = append(results, struct {
                    metricType string
                    occurrence MetricOccurrence
                }{
                    metricType: "Manual/Unlogged Stage",
                    occurrence: MetricOccurrence{
                        InstanceID: instance.ID,
                        Value:      math.Round(percentage*10) / 10,
                        Details:    fmt.Sprintf("Этап '%s': %.1f%% времени (%.2f сек)", instance.Events[i].Description, percentage, stageDuration),
                    },
                })
            }
        }
    }

    return results
}

// collectComplexityMetrics собирает метрики сложности процесса.
func (a *Analyzer) collectComplexityMetrics(instances map[string]*ProcessInstance) []struct {
    metricType string
    occurrence MetricOccurrence
} {
    var results []struct {
        metricType string
        occurrence MetricOccurrence
    }

    uniquePaths := make(map[string]struct{})
    totalInstances := len(instances)

    if totalInstances == 0 {
        return results
    }

    for _, instance := range instances {
        path := ""
        for _, event := range instance.Events {
            path += event.Description + "→"
        }
        if len(path) > 0 {
            path = path[:len(path)-3] // Удаляем последний "→"
        }
        uniquePaths[path] = struct{}{}
    }

    variability := float64(len(uniquePaths)) / float64(totalInstances) * 100

    if variability > 80.0 {
        results = append(results, struct {
            metricType string
            occurrence MetricOccurrence
        }{
            metricType: "High Process Variability",
            occurrence: MetricOccurrence{
                InstanceID: "ALL",
                Value:      math.Round(variability*10) / 10,
                Details:    fmt.Sprintf("%d уникальных путей из %d экземпляров", len(uniquePaths), totalInstances),
            },
        })
    }

    return results
}

// collectCompletionMetrics собирает метрики завершённости процесса.
func (a *Analyzer) collectCompletionMetrics(instances map[string]*ProcessInstance) []struct {
    metricType string
    occurrence MetricOccurrence
} {
    var results []struct {
        metricType string
        occurrence MetricOccurrence
    }

    completedInstances := 0
    totalInstances := len(instances)

    if totalInstances == 0 {
        return results
    }

    for _, instance := range instances {
        if len(instance.Events) < 2 {
            continue
        }

        if len(instance.Events) > 1 &&
		strings.Contains(strings.ToLower(instance.Events[0].Description), "начало") &&
		strings.Contains(strings.ToLower(instance.Events[len(instance.Events)-1].Description), "конец") {
            completedInstances++
        }
    }

    completionRate := float64(completedInstances) / float64(totalInstances) * 100

    if completionRate < 100.0 {
        results = append(results, struct {
            metricType string
            occurrence MetricOccurrence
        }{
            metricType: "Low Process Completion Rate",
            occurrence: MetricOccurrence{
                InstanceID: "ALL",
                Value:      math.Round(completionRate*10) / 10,
                Details:    fmt.Sprintf("%d из %d экземпляров завершены", completedInstances, totalInstances),
            },
        })
    }

    return results
}

// collectErrorMetrics собирает метрики ошибок.
func (a *Analyzer) collectErrorMetrics(instances map[string]*ProcessInstance) []struct {
	metricType string
	occurrence MetricOccurrence
} {
	var results []struct {
		metricType string
		occurrence MetricOccurrence
	}

	errorInstances := 0
	successInstances := 0

	for _, instance := range instances {
		hasError := false
		for _, event := range instance.Events {
			if event.Result == "error" {
				hasError = true
				break
			}
		}
		if hasError {
			errorInstances++
		} else {
			successInstances++
		}
	}

	if errorInstances > successInstances {
		results = append(results, struct {
			metricType string
			occurrence MetricOccurrence
		}{
			metricType: "High Error Rate",
			occurrence: MetricOccurrence{
				InstanceID: "ALL",
				Value:      float64(errorInstances),
				Details:    fmt.Sprintf("%d ошибочных экземпляров против %d успешных", errorInstances, successInstances),
			},
		})
	}

	return results
}

// calculateLinearRegression вычисляет линейную регрессию.
func calculateLinearRegression(data []float64) (slope, intercept float64) {
    var sumX, sumY, sumXY, sumX2 float64
    for i, y := range data {
        x := float64(i)
        sumX += x
        sumY += y
        sumXY += x * y
        sumX2 += x * x
    }

    n := float64(len(data))
    if n == 0 {
        return 0, 0
    }
    slope = (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
    intercept = (sumY - slope*sumX) / n

    return slope, intercept
}

// calculateStandardDeviation вычисляет стандартное отклонение.
func calculateStandardDeviation(data []float64, mean float64) float64 {
    if len(data) < 2 {
        return 0.0
    }

    var sumOfSquares float64
    for _, x := range data {
        sumOfSquares += math.Pow(x-mean, 2)
    }

    return math.Sqrt(sumOfSquares / float64(len(data)-1))
}
