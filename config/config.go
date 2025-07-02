package config

import (
	"fmt"
	"time"
)

const TaskQueueCallbackName = "task_queue_callback"
const TaskQueueName = "task_queue"
const AmqpServerURL = "amqp://admin:admin@localhost:5672/"
const AmqpServerKey = "AMQP_SERVER_URL"

type MessageCode struct {
	IdUser string `json:"id_user"`
	IdTask string `json:"id_task"`
	Code   string `json:"code"`
}

type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Queue   string `json:"queue"`
}

const (
	dbHost     = "localhost"
	dbPort     = 5432
	dbUser     = "root"
	dbPassword = "root"
	dbName     = "mydb"
)

const (
	RedisHost = "localhost:6379"
	RedisPass = "5432"
	RedisDb   = 0
)

var (
	JWTSecretKey  = "your-very-secret-key"
	JWTExpiration = 24 * time.Hour
)

func GetDbUrl() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)
}
