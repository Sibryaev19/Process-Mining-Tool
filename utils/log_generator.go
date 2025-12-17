package utils

import (
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"time"
)

// LogGeneratorConfig содержит параметры для генерации лога.
type LogGeneratorConfig struct {
	OutputFile      string
	NumInstances    int
	MaxEvents       int
	AddSelfLoops    int
	AddPingPongs    int
	AddAnomalies    int
	AddErrors       int
	IncompleteRate  float64
}

// GenerateLog создает CSV-файл с логом процесса на основе конфигурации.
func GenerateLog(config LogGeneratorConfig) error {
	file, err := os.Create(config.OutputFile)
	if err != nil {
		return fmt.Errorf("ошибка создания файла: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Записываем заголовок
	if err := writer.Write([]string{"case_id", "timestamp", "activity", "result"}); err != nil {
		return fmt.Errorf("ошибка записи заголовка: %w", err)
	}

	startTime := time.Now()

	for i := 0; i < config.NumInstances; i++ {
		caseID := fmt.Sprintf("case_%d", i+1)
		events := generateInstance(caseID, startTime, config)
		startTime = startTime.Add(time.Duration(rand.Intn(60)) * time.Minute) // Сдвигаем время для следующего экземпляра

		for _, event := range events {
			record := []string{event.CaseID, event.Timestamp.Format(time.RFC3339), event.Activity, event.Result}
			if err := writer.Write(record); err != nil {
				return fmt.Errorf("ошибка записи события: %w", err)
			}
		}
	}

	return nil
}

// Event представляет событие в генерируемом логе.
type Event struct {
	CaseID    string
	Timestamp time.Time
	Activity  string
	Result    string
}

func generateInstance(caseID string, startTime time.Time, config LogGeneratorConfig) []Event {
	var events []Event
	numEvents := rand.Intn(config.MaxEvents-3) + 3 // От 3 до MaxEvents
	currentTime := startTime

	// Добавляем начальное событие
	events = append(events, Event{caseID, currentTime, "Начало процесса", "success"})

	for j := 1; j < numEvents; j++ {
		activityName := fmt.Sprintf("Этап %c", 'A'+rand.Intn(5))
		currentTime = currentTime.Add(time.Duration(rand.Intn(10)+5) * time.Minute) // +5-15 минут
		events = append(events, Event{caseID, currentTime, activityName, "success"})
	}

	// Внедряем отклонения
	if config.AddSelfLoops > 0 && rand.Float64() < 0.5 {
		events = injectSelfLoop(events)
		config.AddSelfLoops--
	}
	if config.AddPingPongs > 0 && rand.Float64() < 0.5 {
		events = injectPingPong(events)
		config.AddPingPongs--
	}
	if config.AddAnomalies > 0 && rand.Float64() < 0.5 {
		events = injectAnomaly(events)
		config.AddAnomalies--
	}
	if config.AddErrors > 0 && rand.Float64() < 0.5 {
		events = injectError(events)
		config.AddErrors--
	}

	// Добавляем конечное событие (или пропускаем его)
	if rand.Float64() > config.IncompleteRate {
		currentTime = currentTime.Add(time.Duration(rand.Intn(10)+5) * time.Minute)
		events = append(events, Event{caseID, currentTime, "Конец", "success"})
	}

	return events
}

func injectSelfLoop(events []Event) []Event {
	if len(events) < 2 { return events }
	insertIndex := rand.Intn(len(events)-1) + 1
	loopEvent := events[insertIndex-1]
	loopEvent.Timestamp = events[insertIndex-1].Timestamp.Add(1 * time.Minute)
	return append(events[:insertIndex], append([]Event{loopEvent}, events[insertIndex:]...)...)
}

func injectPingPong(events []Event) []Event {
	if len(events) < 2 { return events }
	insertIndex := rand.Intn(len(events)-1) + 1
	ping := events[insertIndex-1]
	pong := events[insertIndex]
	pong.Timestamp = ping.Timestamp.Add(1 * time.Minute)
	ping.Timestamp = pong.Timestamp.Add(1 * time.Minute)
	return append(events[:insertIndex+1], append([]Event{pong, ping}, events[insertIndex+1:]...)...)
}

func injectAnomaly(events []Event) []Event {
	if len(events) < 2 { return events }
	anomalyIndex := rand.Intn(len(events)-1) + 1
	events[anomalyIndex].Timestamp = events[anomalyIndex-1].Timestamp.Add(time.Duration(rand.Intn(120)+60) * time.Minute) // +60-180 минут
	return events
}

func injectError(events []Event) []Event {
	if len(events) == 0 { return events }
	errorIndex := rand.Intn(len(events))
	events[errorIndex].Result = "error"
	return events
}
