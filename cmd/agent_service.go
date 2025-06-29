package main

import (
	"code_runner/config"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/streadway/amqp"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	amqpServerURL := os.Getenv("AMQP_SERVER_URL")
	if amqpServerURL == "" {
		amqpServerURL = "amqp://admin:admin@localhost:5672/"
	}

	connectRabbitMQ, err := amqp.Dial(amqpServerURL)
	if err != nil {
		panic(err)
	}
	defer connectRabbitMQ.Close()

	channelRabbitMQ, err := connectRabbitMQ.Channel()
	if err != nil {
		panic(err)
	}
	defer channelRabbitMQ.Close()

	_, err = channelRabbitMQ.QueueDeclare(
		config.TaskQueueCallbackName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		panic(err)
	}

	messages, err := channelRabbitMQ.Consume(
		config.TaskQueueName,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Println(err)
	}
	log.Println("Successfully connected to RabbitMQ")
	log.Println("Waiting for messages")

	forever := make(chan bool)

	go func() {
		for message := range messages {
			var codeObj config.MessageCode

			err := json.Unmarshal([]byte(message.Body), &codeObj)
			if err != nil {
				log.Printf("Error unmarshaling message: %v", err)
				continue
			}

			uid := uuid.New()
			fileName := uid.String() + ".py"
			dirName := "code"

			// Создаем директорию, если её нет
			if err := os.MkdirAll(dirName, 0755); err != nil {
				log.Printf("Error creating directory: %v", err)
				continue
			}

			if err := makeFile(dirName, fileName, codeObj.Code); err != nil {
				log.Printf("Error creating file: %v", err)
				continue
			}

			success, runMessage := runCode(dirName, fileName)

			// Удаляем файл в любом случае
			if err := deleteFile(dirName, fileName); err != nil {
				log.Printf("Error deleting file: %v", err)
			}

			response := config.SuccessResponse{
				Success: success,
				Message: runMessage,
				Queue:   config.TaskQueueCallbackName,
			}

			jsonBody, err := json.Marshal(response)
			if err != nil {
				log.Printf("Error marshaling response: %v", err)
				continue
			}

			err = channelRabbitMQ.Publish(
				"",
				config.TaskQueueCallbackName,
				false,
				false,
				amqp.Publishing{
					ContentType: "application/json",
					Body:        jsonBody,
				},
			)
			if err != nil {
				log.Printf("Error publishing message: %v", err)
			}
		}
	}()

	<-forever
}

func makeFile(dirName string, fileName string, base64String string) error {
	decodedBytes, err := base64.StdEncoding.DecodeString(base64String)
	if err != nil {
		return fmt.Errorf("base64 decode error: %v", err)
	}

	fullPath := filepath.Join(dirName, fileName)
	err = os.WriteFile(fullPath, decodedBytes, 0644)
	if err != nil {
		return fmt.Errorf("write file error: %v", err)
	}
	return nil
}

func deleteFile(dirName string, fileName string) error {
	fullPath := filepath.Join(dirName, fileName)
	err := os.Remove(fullPath)
	if err != nil {
		return fmt.Errorf("remove file error: %v", err)
	}
	return nil
}

func runCode(dirName string, fileName string) (bool, string) {
	fullPath := filepath.Join(dirName, fileName)

	cmd := exec.Command("python3", fullPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Sprintf("%s\nError: %v", output, err)
	}
	return true, string(output)
}
