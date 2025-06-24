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

type Code struct {
	IdTask   string `json:"id_task"`
	TpRunner string `json:"tp_runner"`
	Code     string `json:"code"`
	Test     string `json:"test"`
}

func main() {
	amqpServerURL := os.Getenv("AMQP_SERVER_URL")
	if amqpServerURL == "" {
		amqpServerURL = "amqp://admin:admin@localhost:5672/" // Default URL if not set
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
		"QueueService1",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		panic(err)
	}

	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	app.Use(
		logger.New(), // add simple logger
	)

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, CORS is enabled!")
	})

	app.Post("/run", func(c *fiber.Ctx) error {
		//todo:  Тестирование ручки на взаимодействие
		// c клиентом. Расширить json добавив ID_USER
		// из AUTH SERVICE

		mess := new(Code)

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

		err = channelRabbitMQ.Publish(
			"",
			"QueueService1",
			false,
			false,
			message,
		)
		if err != nil {
			return err
		}

		return nil
	})

	log.Fatal(app.Listen(":3000"))
}
