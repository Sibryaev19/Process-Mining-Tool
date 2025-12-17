package domain

import (
	"fmt"
	"time"

	"process-mining/internal/domain/metrics"
	"process-mining/internal/infrastructure"
)

type Graph struct {
	Nodes []*Node `json:"nodes"`
	Edges []*Edge `json:"edges"`
}

type Node struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Count int    `json:"count"`
	Total int    `json:"total"`
	Color string `json:"color"`
}

type Edge struct {
	From        string  `json:"from"`
	To          string  `json:"to"`
	Count       int     `json:"count"`
	AvgDuration float64 `json:"-"`
	Label       string  `json:"label"`
	Style       string  `json:"style"` // стиль линии (solid, dashed и т.д.)
}

type Event struct {
	ID        string
	SessionID string
	Timestamp time.Time
	Desc      string
}

type Session struct {
	Events []*Event
}

type GraphBuilder struct {
	graph      *Graph
	nodeMap    map[string]*Node
	edgeMap    map[string]*Edge
	sessionMap map[string]*Session
	csvReader  *infrastructure.CSVReader
}

func NewGraphBuilder(csvReader *infrastructure.CSVReader) *GraphBuilder {
	return &GraphBuilder{
		graph:      &Graph{},
		nodeMap:    make(map[string]*Node),
		edgeMap:    make(map[string]*Edge),
		sessionMap: make(map[string]*Session),
		csvReader:  csvReader,
	}
}

// parseTime пытается разобрать строку времени, используя несколько распространенных форматов.
func parseTime(timeStr string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"02.01.2006 15:04:05",
		"02.01.2006 15:04",
		"2006-01-02T15:04:05Z07:00",
	}

	for _, format := range formats {
		t, err := time.Parse(format, timeStr)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("не удалось распознать формат времени: %s", timeStr)
}

func (gb *GraphBuilder) BuildGraph(filePath string) error {
	err := gb.csvReader.ReadAndProcess(filePath, func(record []string) error {
		// Проверяем, что в записи достаточно столбцов
		if len(record) < 3 {
			return fmt.Errorf("ошибка: запись содержит меньше 3 столбцов: %v", record)
		}

		timestamp, err := parseTime(record[1])
		if err != nil {
			return err // Ошибка уже содержит достаточно контекста
		}

		event := &Event{
			ID:        record[0],
			SessionID: record[0],
			Timestamp: timestamp,
			Desc:      record[2],
		}

		gb.processEvent(event)
		return nil
	})

	if err != nil {
		return err
	}

	gb.finalizeGraph()
	return nil
}

func (gb *GraphBuilder) GetGraph() *Graph {
	return gb.graph
}

func (gb *GraphBuilder) ClearGraph() {
	gb.graph = &Graph{}
	gb.nodeMap = make(map[string]*Node)
	gb.edgeMap = make(map[string]*Edge)
	gb.sessionMap = make(map[string]*Session)
}

func (gb *GraphBuilder) processEvent(event *Event) {
	session := gb.sessionMap[event.SessionID]
	if session == nil {
		session = &Session{}
		gb.sessionMap[event.SessionID] = session
	}
	session.Events = append(session.Events, event)
}

func (gb *GraphBuilder) finalizeGraph() {
	for _, session := range gb.sessionMap {
		gb.processSession(session)
	}

	for _, node := range gb.nodeMap {
		gb.graph.Nodes = append(gb.graph.Nodes, node)
	}

	for _, edge := range gb.edgeMap {
		edge.Label = fmt.Sprintf("%d\n%.2f sec avg", edge.Count, edge.AvgDuration)
		gb.graph.Edges = append(gb.graph.Edges, edge)
	}

	// Добавляем специальные узлы "Начало" и "Конец"
	startNode := &Node{
		ID:    "start",
		Label: "Начало процесса",
		Count: len(gb.sessionMap),
		Total: len(gb.sessionMap),
		Color: "green", // Цвет для начального узла
	}
	gb.graph.Nodes = append(gb.graph.Nodes, startNode)

	endNode := &Node{
		ID:    "end",
		Label: "Конец",
		Count: len(gb.sessionMap),
		Total: len(gb.sessionMap),
		Color: "red", // Цвет для конечного узла
	}
	gb.graph.Nodes = append(gb.graph.Nodes, endNode)

	// Добавляем связи между "Начало" -> первый узел и последний узел -> "Конец"
	for _, session := range gb.sessionMap {
		events := session.Events
		if len(events) == 0 {
			continue
		}

		// Связь "Начало" -> первый узел
		firstEvent := events[0]
		startKey := "start_" + firstEvent.Desc
		startEdge := gb.getEdge(startKey, "start", firstEvent.Desc)
		startEdge.Count++
		startEdge.Style = "dashed" // Устанавливаем стиль линии как пунктирный
		if startEdge.Count == 1 {
			// Если это новая связь, добавляем ее в граф
			gb.graph.Edges = append(gb.graph.Edges, startEdge)
		}

		// Связь последний узел -> "Конец"
		lastEvent := events[len(events)-1]
		endKey := lastEvent.Desc + "_end"
		endEdge := gb.getEdge(endKey, lastEvent.Desc, "end")
		endEdge.Count++
		endEdge.Style = "dashed" // Устанавливаем стиль линии как пунктирный
		if endEdge.Count == 1 {
			// Если это новая связь, добавляем ее в граф
			gb.graph.Edges = append(gb.graph.Edges, endEdge)
		}
	}
}

func (gb *GraphBuilder) processSession(session *Session) {
	events := session.Events
	if len(events) == 0 {
		return
	}

	for _, event := range events {
		node := gb.getNode(event.Desc)
		node.Count++
		node.Total++
	}

	if len(events) > 1 {
		prevEvent := events[0]
		for i := 1; i < len(events); i++ {
			currEvent := events[i]

			duration := currEvent.Timestamp.Sub(prevEvent.Timestamp).Seconds()
			key := prevEvent.Desc + "_" + currEvent.Desc

			edge := gb.getEdge(key, prevEvent.Desc, currEvent.Desc)
			edge.Count++
			edge.AvgDuration = (edge.AvgDuration*float64(edge.Count-1) + duration) / float64(edge.Count)

			prevEvent = currEvent
		}
	}
}

func (gb *GraphBuilder) GetProcessInstances() []metrics.ProcessInstance {
	var processInstances []metrics.ProcessInstance
	for _, session := range gb.sessionMap {
		var events []metrics.Event
		for _, event := range session.Events {
			events = append(events, metrics.Event{
					SessionID: event.SessionID,
					Timestamp: event.Timestamp,
					Description: event.Desc,
				})
			}
			processInstances = append(processInstances, metrics.ProcessInstance{Events: events})
		}
		return processInstances
	}

func (gb *GraphBuilder) getNode(desc string) *Node {
	node := gb.nodeMap[desc]
	if node == nil {
		node = &Node{
			ID:    desc,
			Label: desc,
			Color: "blue", // Устанавливаем значение по умолчанию
		}
		gb.nodeMap[desc] = node
	}
	return node
}

func (gb *GraphBuilder) getEdge(key, from, to string) *Edge {
	edge := gb.edgeMap[key]
	if edge == nil {
		edge = &Edge{
			From: from,
			To:   to,
		}
		gb.edgeMap[key] = edge
	}
	return edge
}
