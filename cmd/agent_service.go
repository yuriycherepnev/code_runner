package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"log"
	"os"
	"os/exec"

	"github.com/streadway/amqp"
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
			log.Printf(" > Received message: %s\n", message.Body)

			var codeObj Code

			err := json.Unmarshal([]byte(message.Body), &codeObj)
			if err != nil {
				log.Fatalf("Ошибка при разборе JSON: %v", err)
			}

			uid := uuid.New()
			dirName := uid.String()
			makeDir(dirName)
			makeFile(dirName, codeObj.Code, "main.py")
			//TODO написать тест для кода
			//mkfile(dirName, code_obj.Test, "test_hello.py")
			//run(dirName)
			fmt.Println(codeObj.Code)

		}
	}()

	<-forever
}

func makeDir(dirName string) {
	permissions := os.ModeDir | 0755
	err := os.Mkdir(dirName, permissions)
	if err != nil {
		log.Printf("Ошибка при создании директории: %v", err)
	}
	fmt.Printf("Директория '%s' успешно создана.\n", dirName)
}

func makeFile(dirName string, base64String string, fileName string) {
	decodedBytes, er := base64.StdEncoding.DecodeString(base64String)
	if er != nil {
		log.Fatalf("Ошибка при декодировании Base64: %v", er)
	}
	fileContent := string(decodedBytes)
	fullName := "./" + dirName + "/" + fileName
	permissions := 0644
	data := []byte(fileContent)
	err := os.WriteFile(fullName, data, os.FileMode(permissions))
	if err != nil {
		log.Fatalf("Ошибка при создании файла: %v", err)
	}
	fmt.Printf("Файл '%s' успешно создан с содержимым.\n", fileName)
}

func run(taskDir string) {
	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Ошибка при получении текущей директории: %v", err)
	}
	dir := currentDir + "/" + taskDir
	err = os.Chdir(dir)
	if err != nil {
		log.Fatalf("Ошибка при изменении директории: %v", err)
	}
	fileName := "test_hello.py"
	cmd := exec.Command("python", "-m", "unittest", fileName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Ошибка при выполнении Python кода: %v\nВывод: %s", err, string(output))
	}

	// Вывод результата
	// todo: записать в топик RMQ id_user_id_UUID
	// {  data: output}
	fmt.Printf("Вывод Python:\n%s\n", string(output))

}
