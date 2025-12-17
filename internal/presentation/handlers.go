package presentation

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"process-mining/internal/domain"
	"process-mining/internal/infrastructure"
	"process-mining/internal/service"
)

type GraphHandler struct {
	graphService *service.GraphService
}

func NewGraphHandler(graphService *service.GraphService) *GraphHandler {
	return &GraphHandler{graphService: graphService}
}

func (h *GraphHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	log.Println("Начало обработки запроса на загрузку файла")

	cleaner := infrastructure.NewTMPCleaner()
	if err := cleaner.ClearTempFiles(); err != nil {
		log.Printf("Ошибка очистки временных файлов: %v", err)
	}

	if r.Method != http.MethodPost {
		log.Println("Метод не поддерживается")
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 3*1024*1024*1024) // 3 ГБ
	file, _, err := r.FormFile("file")
	if err != nil {
		log.Printf("Ошибка получения файла: %v", err)
		http.Error(w, "Ошибка загрузки файла", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tempFile, err := os.CreateTemp("", "uploaded-*.csv")
	if err != nil {
		log.Printf("Ошибка создания временного файла: %v", err)
		http.Error(w, "Ошибка создания временного файла", http.StatusInternalServerError)
		return
	}
	defer tempFile.Close()

	buf := make([]byte, 1024*1024) // Буфер размером 1 МБ
	for {
		n, err := file.Read(buf)
		if n > 0 {
			if _, writeErr := tempFile.Write(buf[:n]); writeErr != nil {
				log.Printf("Ошибка записи во временный файл: %v", writeErr)
				http.Error(w, "Ошибка записи во временный файл", http.StatusInternalServerError)
				return
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Ошибка чтения файла: %v", err)
			http.Error(w, "Ошибка чтения файла", http.StatusInternalServerError)
			return
		}
	}

	log.Println("Файл успешно загружен. Начинается обработка...")
	err = h.graphService.BuildGraphFromCSV(tempFile.Name())
	if err != nil {
		log.Printf("Ошибка построения графа: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка построения графа: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Файл успешно загружен и граф построен"))
	log.Println("Обработка завершена успешно")
}

func (h *GraphHandler) ServeGraphData(w http.ResponseWriter, r *http.Request) {
	graphData, err := h.graphService.GetGraphData()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// // Логирование данных для отладки
	// for _, edge := range graphData.Edges {
	// 	fmt.Printf("Edge: %s -> %s, Style: %s\n", edge.From, edge.To, edge.Style)
	// }

	// Преобразуем данные в формат, понятный фронтенду
	cytoscapeData := struct {
		Nodes []map[string]*domain.Node `json:"nodes"`
		Edges []map[string]*domain.Edge `json:"edges"`
	}{
		Nodes: make([]map[string]*domain.Node, len(graphData.Nodes)),
		Edges: make([]map[string]*domain.Edge, len(graphData.Edges)),
	}

	for i, node := range graphData.Nodes {
		cytoscapeData.Nodes[i] = map[string]*domain.Node{"data": node}
	}

	for i, edge := range graphData.Edges {
		edge.Label = fmt.Sprintf("%d\n%.2f sec avg", edge.Count, edge.AvgDuration)
		cytoscapeData.Edges[i] = map[string]*domain.Edge{"data": edge}
	}

	// Отправляем данные клиенту
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cytoscapeData); err != nil {
		http.Error(w, "Ошибка сериализации", http.StatusInternalServerError)
		return
	}
}

func (h *GraphHandler) ClearGraph(w http.ResponseWriter, r *http.Request) {
	cleaner := infrastructure.NewTMPCleaner()
	if err := cleaner.ClearTempFiles(); err != nil {
		log.Printf("Ошибка очистки временных файлов: %v", err)
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	h.graphService.ClearGraph()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Граф успешно очищен"))
}

func (h *GraphHandler) GetMetricsReport(w http.ResponseWriter, r *http.Request) {
	log.Println("Начало обработки запроса на получение отчета по метрикам")

	metricsReport, err := h.graphService.GetMetricsReport()
	if err != nil {
		log.Printf("Ошибка получения отчета по метрикам: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка получения отчета по метрикам: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metricsReport); err != nil {
		log.Printf("Ошибка сериализации отчета по метрикам: %v", err)
		http.Error(w, "Ошибка сериализации отчета по метрикам", http.StatusInternalServerError)
		return
	}
	// Логирование JSON-ответа перед отправкой
	jsonOutput, _ := json.MarshalIndent(metricsReport, "", "  ")
	log.Printf("Отправляемый JSON-отчет по метрикам:\n%s", jsonOutput)
	log.Println("Отчет по метрикам успешно отправлен")
}
