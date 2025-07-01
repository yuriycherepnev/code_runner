package main

import (
	"code_runner/config"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type UserResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type AuthResponse struct {
	User  UserResponse `json:"user"`
	Token string       `json:"token"`
}

var (
	postgresDb *sql.DB
	redisDb    *redis.Client
)

func initDb() error {
	connStr := config.GetDbUrl()
	var err error
	postgresDb, err = sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	return postgresDb.Ping()
}

func initRedis() error {
	redisDb = redis.NewClient(&redis.Options{
		Addr:     config.RedisHost,
		Password: config.RedisPass,
		DB:       config.RedisDb,
	})

	ctx := context.Background()
	_, err := redisDb.Ping(ctx).Result()
	return err
}

func registerUser(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	var id int
	err = postgresDb.QueryRow(`
		INSERT INTO "user" (name, email, password) 
		VALUES ($1, $2, $3) 
		RETURNING id`, user.Name, user.Email, string(hashedPassword)).Scan(&id)
	if err != nil {
		fmt.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user: " + err.Error()})
		return
	}

	token := generateToken()
	if err != nil {
		log.Printf("Failed to save token to Redis: %v", err)
	}

	response := AuthResponse{
		User: UserResponse{
			ID:    id,
			Name:  user.Name,
			Email: user.Email,
		},
		Token: token,
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"data":    response,
	})
}

func loginUser(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user struct {
		ID       int
		Name     string
		Email    string
		Password string
	}

	err := postgresDb.QueryRow(`
        SELECT id, name, email, password 
        FROM "user" 
        WHERE email = $1`, req.Email).
		Scan(&user.ID, &user.Name, &user.Email, &user.Password)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	fmt.Print(string(hashedPassword))

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	token := generateToken()

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"data": AuthResponse{
			User: UserResponse{
				ID:    user.ID,
				Name:  user.Name,
				Email: user.Email,
			},
			Token: token,
		},
	})
}

func generateToken() string {
	return "generated_token_" + fmt.Sprint(time.Now().UnixNano())
}

func main() {
	if err := initDb(); err != nil {
		log.Fatalf("PostgreSQL init error: %v", err)
	}
	defer postgresDb.Close()

	router := gin.Default()
	router.LoadHTMLGlob("./templates/*")

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "auth.html", gin.H{
			"idUser":   22,
			"userName": "Yuriy",
		})
	})

	router.POST("/register", registerUser)
	router.POST("/login", loginUser)

	if err := router.Run(":8081"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
