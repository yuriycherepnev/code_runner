package main

import (
	"fmt"
	"log"
	"os"

	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/streadway/amqp"
)

// Структура для JSON данных
type Code struct {
	Id_task   string `json:"id_task"`
	Tp_runner string `json:"tp_runner"`
	Code      string `json:"code"`
	Test      string `json:"test"`
}

func main() {
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

	// Let's start by opening a channel to our RabbitMQ
	// instance over the connection we have already
	// established.
	channelRabbitMQ, err := connectRabbitMQ.Channel()
	if err != nil {
		panic(err)
	}
	defer channelRabbitMQ.Close()

	// With the instance and declare Queues that we can
	// publish and subscribe to.
	_, err = channelRabbitMQ.QueueDeclare(
		"QueueService1", // queue name
		true,            // durable
		false,           // auto delete
		false,           // exclusive
		false,           // no wait
		nil,             // arguments
	)
	if err != nil {
		panic(err)
	}

	// Create a new Fiber instance.
	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	// Add middleware.
	app.Use(
		logger.New(), // add simple logger
	)

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, CORS is enabled!")
	})
	// Add route.  ?msg=bla

	app.Post("/run", func(c *fiber.Ctx) error {
		//todo:  Тестирование ручки на взаимодействие
		// c клиентом. Расширить json добавив ID_USER 
		// из AUTH SERVICE

		// Create a message to publish.
		// to-do: дописать приемку json
		mess := new(Code)

		// Разбираем JSON из тела запроса и заполняем структуру
		_ = c.BodyParser(mess)
		jsonBody, err := json.Marshal(mess)
		if err != nil {
			log.Fatalf("Error encoding JSON: %v", err)
		}

		fmt.Println(mess.Code)
		message := amqp.Publishing{
			ContentType: "application/json",
			Body:        jsonBody,
		}

		// message := amqp.Publishing{
		//     ContentType: "text/plain",
		//     Body:        []byte("Hello"),
		// }

		// Attempt to publish a message to the queue.
		if err := channelRabbitMQ.Publish(
			"",              // exchange
			"QueueService1", // queue name
			false,           // mandatory
			false,           // immediate
			message,         // message to publish
		); err != nil {
			return err
		}

		return nil
	})

	// Start Fiber API server.
	log.Fatal(app.Listen(":3000"))
}
