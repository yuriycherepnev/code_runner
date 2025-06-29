package main

import (
	"code_runner/config"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/streadway/amqp"
	"log"
	"net/http"
	"os"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	type CallbackMessage struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Queue   string `json:"queue"`
	}

	ws, _ := upgrader.Upgrade(w, r, nil)

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
		config.TaskQueueCallbackName,
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

	if err != nil {
		log.Fatal(err)
	}

	defer ws.Close()

	forever := make(chan bool)

	go func() {
		for message := range messages {
			log.Printf(" > Received message: %s\n", message.Body)
			var messageObj CallbackMessage

			err := json.Unmarshal([]byte(message.Body), &messageObj)
			if err != nil {
				log.Fatalf("Ошибка при разборе JSON: %v", err)
			}
			err = ws.WriteMessage(websocket.TextMessage, []byte(messageObj.Message))
			if err != nil {
				log.Println(err)
				return
			}
		}
	}()

	for {
		_, msg, err := ws.ReadMessage()
		fmt.Println(err)

		if err != nil {
			return
		}
		fmt.Println(err)

		err = ws.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			return
		}
	}

	<-forever
}

func main() {
	fmt.Println("Starting WebSocket server...")
	http.HandleFunc("/ws", handleConnections)
	log.Fatal(http.ListenAndServe(":8082", nil))
}
