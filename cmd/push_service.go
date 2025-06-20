package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"os"
	"github.com/streadway/amqp"
	"github.com/gorilla/websocket"
)



var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins (for development).  In production, specify allowed origins.
	},
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	
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
	

    // todo: заменен QueueService1 на динамический
	// id_user_id_task 
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

     	// Upgrade initial GET request to a websocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	// Make sure we close the connection when the function returns
	defer ws.Close()


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
			// Write message back to browser
			err = ws.WriteMessage(websocket.TextMessage, []byte(code_obj.Code))
			if err != nil {
				log.Println(err)
				return
			}
				fmt.Println(code_obj.Code)

		}
	}()

	<-forever


	//  for {
	// 	// Read message from browser
	// 	_, msg, err := ws.ReadMessage()
	// 	if err != nil {
	// 		log.Println(err)
	// 		return
	// 	}

	// 	// Print the message to the console
	// 	fmt.Printf("Received: %s\n", msg)

	// 	// Write message back to browser
	// 	err = ws.WriteMessage(websocket.TextMessage, msg)
	// 	if err != nil {
	// 		log.Println(err)
	// 		return
	// 	}
	// }
}

func main() {
	
	fmt.Println("Starting WebSocket server...")
	http.HandleFunc("/ws", handleConnections)

	log.Fatal(http.ListenAndServe(":8081", nil))
}