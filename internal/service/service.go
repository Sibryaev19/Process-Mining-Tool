package service

import (
	"process-mining/internal/domain"
	"process-mining/internal/domain/metrics"
)

type GraphService struct {
	graphBuilder *domain.GraphBuilder
}

func NewGraphService(graphBuilder *domain.GraphBuilder) *GraphService {
	return &GraphService{graphBuilder: graphBuilder}
}

func (s *GraphService) BuildGraphFromCSV(filePath string) error {
	return s.graphBuilder.BuildGraph(filePath)
}

func (s *GraphService) GetGraphData() (*domain.Graph, error) {
	return s.graphBuilder.GetGraph(), nil
}

func (s *GraphService) ClearGraph() {
	s.graphBuilder.ClearGraph()
}

func (s *GraphService) GetMetricsReport() (*metrics.MetricsReport, error) {
	analyzer := metrics.NewAnalyzer()
	processInstancesSlice := s.graphBuilder.GetProcessInstances()

	// Конвертируем слайс в мапу для анализатора
	processInstancesMap := make(map[string]*metrics.ProcessInstance)
	for i := range processInstancesSlice {
		domainPI := processInstancesSlice[i]

		// Конвертируем события
		metricEvents := make([]metrics.Event, len(domainPI.Events))
		for j, event := range domainPI.Events {
			metricEvents[j] = metrics.Event{
				SessionID:   event.SessionID,
				Timestamp:   event.Timestamp,
				Description: event.Description,
				Result:      event.Result,
			}
		}

		// Создаем и добавляем экземпляр процесса для метрик
		processInstancesMap[domainPI.ID] = &metrics.ProcessInstance{
			ID:     domainPI.ID,
			Events: metricEvents,
		}
	}

	return analyzer.Analyze(processInstancesMap), nil
}
