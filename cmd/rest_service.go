package main

import (
	"code_runner/config"
	"fmt"
	"github.com/gofiber/fiber/v2"
	_ "github.com/gofiber/fiber/v2"
	"log"
	"os"

	"encoding/json"

	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/streadway/amqp"
)

func main() {
	amqpServer := os.Getenv(config.AmqpServerKey)
	if amqpServer == "" {
		amqpServer = config.AmqpServerURL
	}

	connectRabbitMQ, err := amqp.Dial(amqpServer)
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
		config.TaskQueueName,
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
		logger.New(),
	)

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, CORS is enabled!")
	})

	app.Post("/run", func(c *fiber.Ctx) error {
		mess := new(config.MessageCode)

		if err := c.BodyParser(mess); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Invalid request body",
			})
		}

		jsonBody, err := json.Marshal(mess)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Error encoding message",
			})
		}

		fmt.Println("Received message:", mess)

		message := amqp.Publishing{
			ContentType: "application/json",
			Body:        jsonBody,
		}

		err = channelRabbitMQ.Publish(
			"",
			config.TaskQueueName,
			false,
			false,
			message,
		)

		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to publish message to queue",
			})
		}

		response := config.SuccessResponse{
			Success: true,
			Message: "Code successfully added to queue",
			Queue:   config.TaskQueueName,
		}

		return c.Status(fiber.StatusOK).JSON(response)
	})

	log.Fatal(app.Listen(":3000"))
}
