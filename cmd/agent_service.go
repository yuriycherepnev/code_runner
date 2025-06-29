package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/streadway/amqp"
	"log"
	"os"
	"os/exec"
)

const taskQueueCallbackName = "task_queue_callback"
const taskQueueName = "task_queue"
const amqpServerURL = "amqp://admin:admin@localhost:5672/"
const amqpServerKey = "AMQP_SERVER_URL"

type Code struct {
	IdUser string `json:"id_user"`
	IdTask string `json:"id_task"`
	Code   string `json:"code"`
}

type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Queue   string `json:"queue"`
}

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

	_, err = channelRabbitMQ.QueueDeclare(
		taskQueueCallbackName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		panic(err)
	}

	defer channelRabbitMQ.Close()

	messages, err := channelRabbitMQ.Consume(
		taskQueueName,
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
			var codeObj Code

			err := json.Unmarshal([]byte(message.Body), &codeObj)
			if err != nil {
				log.Fatalf("error: %v", err)
			}
			uid := uuid.New()
			fileName := uid.String() + ".py"
			dirName := "code"
			makeFile(dirName, fileName, codeObj.Code)
			runCode(dirName, fileName)

			response := SuccessResponse{
				Success: true,
				Message: "Code successfully run",
				Queue:   taskQueueCallbackName,
			}
			jsonBody, err := json.Marshal(response)

			fmt.Println(jsonBody)
			message := amqp.Publishing{
				ContentType: "application/json",
				Body:        jsonBody,
			}

			err = channelRabbitMQ.Publish(
				"",
				taskQueueCallbackName,
				false,
				false,
				message,
			)
		}
	}()

	<-forever
}

func makeFile(dirName string, fileName string, base64String string) {
	decodedBytes, er := base64.StdEncoding.DecodeString(base64String)
	if er != nil {
		log.Fatalf("makeFile error: %v", er)
	}
	fileContent := string(decodedBytes)
	fullName := "./" + dirName + "/" + fileName
	permissions := 0644
	data := []byte(fileContent)
	err := os.WriteFile(fullName, data, os.FileMode(permissions))
	if err != nil {
		return
	}
}

func runCode(dirName string, fileName string) (bool, string) {
	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Ошибка при получении текущей директории: %v", err)
	}
	dir := currentDir + "/" + dirName
	err = os.Chdir(dir)
	if err != nil {
		log.Fatalf("Ошибка при изменении директории: %v", err)
	}
	//fileName := "test_hello.py"
	//cmd := exec.Command("python", "-m", "unittest", fileName)

	cmd := exec.Command("python3", fileName)
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Fatalf("Ошибка при выполнении Python кода: %v\nВывод: %s", err, string(output))
	}

	// Вывод результата
	// todo: записать в топик RMQ id_user_id_UUID
	// {  data: output}

	return true, string(output)

}
