package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/streadway/amqp"
)

func main() {

	type Code struct {
		IDTask   string `json:"id_task"`
		TpRunner string `json:"tp_runner"`
		Code     string `json:"code"`
		Test     string `json:"test"`
	}

	// Define RabbitMQ server URL.
	amqpServerURL := os.Getenv("AMQP_SERVER_URL")
	if amqpServerURL == "" {
		amqpServerURL = "amqp://guest:guest@localhost:5672/" // Default URL if not set
	}

	// Create a new RabbitMQ connection.
	connectRabbitMQ, err := amqp.Dial(amqpServerURL)
	if err != nil {
		panic(err)
	}
	defer connectRabbitMQ.Close()

	// Opening a channel to our RabbitMQ instance over
	// the connection we have already established.
	channelRabbitMQ, err := connectRabbitMQ.Channel()
	if err != nil {
		panic(err)
	}
	defer channelRabbitMQ.Close()

	// Subscribing to QueueService1 for getting messages.
	messages, err := channelRabbitMQ.Consume(
		"QueueService1", // queue name
		"",              // consumer
		true,            // auto-ack
		false,           // exclusive
		false,           // no local
		false,           // no wait
		nil,             // arguments
	)
	if err != nil {
		log.Println(err)
	}

	// Build a welcome message.
	log.Println("Successfully connected to RabbitMQ")
	log.Println("Waiting for messages")

	// Make a channel to receive messages into infinite loop.
	forever := make(chan bool)

	go func() {
		for message := range messages {
			// For example, show received message in a console.
			log.Printf(" > Received message: %s\n", message.Body)
			// Создание экземпляра структуры Task

			var code_obj Code

			// Разбор JSON строки в структуру
			err := json.Unmarshal([]byte(message.Body), &code_obj)
			if err != nil {
				log.Fatalf("Ошибка при разборе JSON: %v", err)
			}
			mkdir(code_obj.IDTask)
			mkfile(code_obj.IDTask, code_obj.Code, "main.py")
			mkfile(code_obj.IDTask, code_obj.Test, "test_hello.py")
			run(code_obj.IDTask)

			fmt.Println(code_obj.Code)

		}
	}()

	<-forever
}

func mkdir(dirName string) {
	// Имя директории, которую нужно создать

	// Права доступа к директории (0755 - чтение, запись, выполнение для владельца, чтение и выполнение для группы и остальных)
	permissions := os.ModeDir | 0755

	// Создание директории
	err := os.Mkdir(dirName, permissions)
	if err != nil {
		log.Printf("Ошибка при создании директории: %v", err)
	}

	fmt.Printf("Директория '%s' успешно создана.\n", dirName)
}

func mkfile(dirName string, base64String string, fName string) {

	// Декодирование Base64 строки
	decodedBytes, er := base64.StdEncoding.DecodeString(base64String)
	if er != nil {
		log.Fatalf("Ошибка при декодировании Base64: %v", er)
	}

	// Преобразование байтов в строку
	fileContent := string(decodedBytes)

	// Права доступа к файлу (0644 - чтение и запись для владельца, чтение для группы и остальных)
	fileName := "./" + dirName + "/" + fName 
	permissions := 0644

	// Преобразование содержимого в слайс байтов
	data := []byte(fileContent)

	// Создание файла и запись содержимого
	err := os.WriteFile(fileName, data, os.FileMode(permissions))
	if err != nil {
		log.Fatalf("Ошибка при создании файла: %v", err)
	}

	fmt.Printf("Файл '%s' успешно создан с содержимым.\n", fileName)

}

func run(idTask string) {

	// Получение текущей рабочей директории
	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Ошибка при получении текущей директории: %v", err)
	}

	dir := currentDir + "/" + idTask

    // Изменение текущей рабочей директории
	err = os.Chdir(dir)
	if err != nil {
		log.Fatalf("Ошибка при изменении директории: %v", err)
	}

	// Команда для запуска Python с кодом
	fileName := "test_hello.py"
	
	
	cmd := exec.Command("python", "-m", "unittest", fileName)

	// Запуск команды и получение вывода
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Ошибка при выполнении Python кода: %v\nВывод: %s", err, string(output))
	}

	// Вывод результата
	// todo: записать в топик RMQ id_user_id_UUID 
	// {  data: output}
	fmt.Printf("Вывод Python:\n%s\n", string(output))

}
