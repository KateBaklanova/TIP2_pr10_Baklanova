package http

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"kate/services/tasks/internal/cache"
	"kate/services/tasks/internal/client"
	"kate/services/tasks/internal/http/handler"
	"kate/services/tasks/internal/service"
	"kate/shared/middleware"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func StartServer(port string, authGrpcAddr string, logger *zap.Logger, instanceID string) {
	redisClient := cache.NewRedisClient("redis:6379")
	if err := cache.Ping(context.Background(), redisClient); err != nil {
		log.Println("[WARN] Redis unavailable:", err)
		redisClient = nil
	} else {
		log.Println("[INFO] Redis connected")
	}

	var taskSvc *service.TaskService
	if redisClient != nil {
		taskSvc = service.NewTaskServiceWithRedis(redisClient)
	} else {
		taskSvc = service.NewTaskService()
	}

	authGrpc, err := client.NewAuthGrpcClient(authGrpcAddr, logger)
	if err != nil {
		logger.Fatal("Failed to connect to Auth gRPC", zap.Error(err))
	}
	defer authGrpc.Close()

	taskHandler := handler.NewTaskHandler(taskSvc, authGrpc, logger)

	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Instance-ID", instanceID)
		json.NewEncoder(w).Encode(map[string]string{
			"status":   "ok",
			"instance": instanceID,
		})
	})

	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/v1/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Instance-ID", instanceID)
		switch r.Method {
		case http.MethodGet:
			taskHandler.GetAllTasks(w, r)
		case http.MethodPost:
			taskHandler.CreateTask(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/v1/tasks/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Instance-ID", instanceID)
		switch r.Method {
		case http.MethodGet:
			taskHandler.GetTaskByID(w, r)
		case http.MethodPatch:
			taskHandler.UpdateTask(w, r)
		case http.MethodDelete:
			taskHandler.DeleteTask(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	handlerWithMiddleware := middleware.RequestIDMiddleware(
		middleware.MetricsMiddleware(
			middleware.LoggingMiddleware(logger)(mux),
		),
	)

	logger.Info("Tasks HTTP server starting", zap.String("port", port), zap.String("instance_id", instanceID))
	if err := http.ListenAndServe(":"+port, handlerWithMiddleware); err != nil {
		logger.Fatal("HTTP server failed", zap.Error(err))
	}
}
