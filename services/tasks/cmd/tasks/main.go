package main

import (
	"log"
	"os"

	http2 "kate/services/tasks/internal/http"
	"kate/shared/logger"
)

type Task struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

func main() {
	instanceID := os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		instanceID = "tasks-unknown"
	}

	port := os.Getenv("TASKS_PORT")
	if port == "" {
		port = "8082"
	}

	authGrpcAddr := os.Getenv("AUTH_GRPC_ADDR")
	if authGrpcAddr == "" {
		authGrpcAddr = "localhost:50051"
	}

	logger, err := logger.New("tasks")
	if err != nil {
		log.Fatalf("Failed to init logger: %v", err)
	}
	defer logger.Sync()

	http2.StartServer(port, authGrpcAddr, logger, instanceID)
}
