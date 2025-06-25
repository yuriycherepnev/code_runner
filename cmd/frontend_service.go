package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

const (
	dbHost     = "localhost"
	dbPort     = 5432
	dbUser     = "root"
	dbPassword = "root"
	dbName     = "mydb"
)

func main() {
	redisDb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Адрес Redis сервера
		Password: "",               // Пароль, если требуется
		DB:       0,                // Номер базы данных
	})

	ctx := context.Background()
	pong, err := redisDb.Ping(ctx).Result()
	if err != nil {
		fmt.Println("Ошибка подключения к Redis:", err)
		return
	}

	fmt.Println("Успешное подключение к Redis! Ответ:", pong)

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	postgresDb, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer postgresDb.Close()

	err = postgresDb.Ping()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to the database!")

	router := gin.Default()
	router.LoadHTMLGlob("./templates/*")

	router.GET("/task/:id", func(c *gin.Context) {
		var code string

		id := c.Param("id")
		code, err := redisDb.Get(ctx, id).Result()
		if err != nil {
			err := postgresDb.QueryRow("SELECT code FROM task WHERE id  = $1", id).Scan(&code)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			// Пример использования: установка и получение значения
			err = redisDb.Set(ctx, id, code, 0).Err()
			if err != nil {
				fmt.Println("Ошибка при установке значения:", err)
				return
			}
			fmt.Println("Установили значение по ключу:", err)
		}
		c.JSON(http.StatusOK, gin.H{"code": code})
	})

	router.POST("/task/:id", func(c *gin.Context) {
		id := c.Param("id")
		var requestBody struct {
			Code string `json:"code" binding:"required"`
		}

		if err := c.BindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err := postgresDb.Exec("UPDATE task SET code = $1 WHERE id = $2", requestBody.Code, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Snippet updated successfully"})
	})

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	})

	// Запуск сервера
	router.Run(":8081")
}
