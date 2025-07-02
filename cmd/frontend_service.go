package main

import (
	"code_runner/config"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
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

type Claims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

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

func generateToken(userID int) (string, error) {
	expirationTime := time.Now().Add(config.JWTExpiration)

	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "your-app-name",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.JWTSecretKey))
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(config.JWTSecretKey), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Next()
	}
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

	token, err := generateToken(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
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
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	token, err := generateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

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

	// Защищенный маршрут с использованием JWT
	router.GET("/profile", authMiddleware(), func(c *gin.Context) {
		userID, exist := c.Get("user_id")
		if !exist {
			return
		}
		c.HTML(http.StatusOK, "profile.html", gin.H{
			"idUser":   userID,
			"userName": "Yuriy",
		})
	})

	router.POST("/register", registerUser)
	router.POST("/login", loginUser)

	if err := router.Run(":8081"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
