package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq" // Import the PostgreSQL driver
	"github.com/redis/go-redis/v9"
)

const (
	dbHost     = "localhost"
	dbPort     = 5432
	dbUser     = "postgres"
	dbPassword = "123" // Замените на свой пароль
	dbName     = "auth"
)

func main() {

	// Создание клиента Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Адрес Redis сервера
		Password: "",               // Пароль, если требуется
		DB:       0,                // Номер базы данных
	})

	// Проверка подключения
	ctx := context.Background()
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		fmt.Println("Ошибка подключения к Redis:", err)
		return
	}

	fmt.Println("Успешное подключение к Redis! Ответ:", pong)

	// Строка подключения к базе данных
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	// to-do: написать обращение в кеш  Redis
	// Подключение к базе данных
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Проверка подключения
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to the database!")

	router := gin.Default()
	router.LoadHTMLGlob("./templates/*")

	// Обработчик для получения фрагмента кода
	router.GET("/task/:id", func(c *gin.Context) {
		var code string

		id := c.Param("id")
		code, err := rdb.Get(ctx, id).Result()
		if err != nil {
			err := db.QueryRow("SELECT json_text FROM task WHERE id  = $1", id).Scan(&code)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			// Пример использования: установка и получение значения
			err = rdb.Set(ctx, id, code, 0).Err()
			if err != nil {
				fmt.Println("Ошибка при установке значения:", err)
				return
			}
			fmt.Println("Установили значение по ключу:", err)
		}
		c.JSON(http.StatusOK, gin.H{"code": code})
	})

	// Обработчик для обновления фрагмента кода
	router.POST("/task/:id", func(c *gin.Context) {
		id := c.Param("id")
		var requestBody struct {
			Code string `json:"code" binding:"required"`
		}

		if err := c.BindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_, err := db.Exec("UPDATE snippets SET code = $1 WHERE id = $2", requestBody.Code, id)
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
	router.Run(":8080")
}
