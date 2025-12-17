package cmd

import (
	"log"
	"net/http"
	"time"

	"process-mining/config"
	"process-mining/internal/domain"
	"process-mining/internal/infrastructure"
	"process-mining/internal/presentation"
	"process-mining/internal/service"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Запуск HTTP-сервера",
	Long:  "Запускает HTTP-сервер для обработки запросов.",
	Run: func(cmd *cobra.Command, args []string) {
		// Инициализация инфраструктурного слоя
		csvReader := infrastructure.NewCSVReader()

		// Инициализация доменного слоя
		graphBuilder := domain.NewGraphBuilder(csvReader)

		// Инициализация сервисного слоя
		graphService := service.NewGraphService(graphBuilder)

		// Инициализация слоя представления
		graphHandler := presentation.NewGraphHandler(graphService)

		// Настройка маршрутов
		http.Handle("/", http.FileServer(http.Dir("./static"))) // Статические файлы
		http.HandleFunc("/upload", graphHandler.UploadFile)     // Загрузка CSV
		http.HandleFunc("/graph", graphHandler.ServeGraphData)  // Получение данных графа
		http.HandleFunc("/clear", graphHandler.ClearGraph)      // Очистка графа
		http.HandleFunc("/metrics", graphHandler.GetMetricsReport) // Получение отчета по метрикам

		cfg, err := config.LoadEnv()
		if err != nil {
			log.Fatalln("can not load config", err)
		}

		// Настройка сервера с увеличенными таймаутами
		srv := &http.Server{
			Addr:         ":" + cfg.APP_PORT,
			WriteTimeout: cfg.GetAppMaxWriteTime() * time.Minute, // Увеличенный таймаут для записи
			ReadTimeout:  cfg.GetAppMaxReadTime() * time.Minute,  // Увеличенный таймаут для чтения
		}

		// Логирование запуска сервера
		log.Printf("Сервер запущен на порту %v", srv.Addr)

		// Запуск сервера
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка запуска сервера: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
