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
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type Task struct {
	ID     int    `json:"id"`
	Text   string `json:"text"`
	IdLang int    `json:"id_lang"`
}

var user struct {
	ID    int
	Name  string
	Email string
}

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

type SolutionRequest struct {
	TaskID string `json:"task_id" binding:"required"`
	Code   string `json:"code" binding:"required"`
}

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

func extractTokenFromHeader(c *gin.Context) (string, error) {
	tokenString := c.GetHeader("Authorization")
	if tokenString == "" {
		return "", errors.New("authorization header is required")
	}

	if len(tokenString) > 7 && strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = tokenString[7:]
	}

	return tokenString, nil
}

func extractTokenFromCookie(c *gin.Context) (string, error) {
	tokenString, err := c.Cookie("jwtToken")
	if err != nil {
		return "", errors.New("jwt cookie is required")
	}
	return tokenString, nil
}

func parseToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.JWTSecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := extractTokenFromHeader(c)
		if err != nil {
			tokenString, err = extractTokenFromCookie(c)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token is required"})
				c.Abort()
				return
			}
		}

		claims, err := parseToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: " + err.Error()})
			c.Abort()
			return
		}

		err = postgresDb.QueryRow(`
			SELECT id, name, email 
			FROM "user" 
			WHERE id = $1`, claims.UserID).
			Scan(&user.ID, &user.Name, &user.Email)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			c.Abort()
			return
		}

		c.Set("user_id", user.ID)
		c.Set("user_name", user.Name)
		c.Set("user_email", user.Email)

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

	c.SetCookie("jwtToken", token, int(config.JWTExpiration.Seconds()), "/", "", false, true)

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

	c.SetCookie("jwtToken", token, int(config.JWTExpiration.Seconds()), "/", "", false, true)

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

func getAllTask(c *gin.Context) {
	rows, err := postgresDb.Query(`
        SELECT id, text, id_lang
        FROM task`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tasks: " + err.Error()})
		return
	}
	defer rows.Close()

	var taskList []Task

	for rows.Next() {
		var task Task
		err := rows.Scan(
			&task.ID,
			&task.Text,
			&task.IdLang,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan task: " + err.Error()})
			return
		}
		taskList = append(taskList, task)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error after scanning rows: " + err.Error()})
		return
	}

	c.HTML(http.StatusOK, "task_list.html", gin.H{
		"task_list": taskList,
	})
}

func getTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	var task Task
	err = postgresDb.QueryRow(`
        SELECT id, text, id_lang
        FROM task
        WHERE id = $1`, id).
		Scan(
			&task.ID,
			&task.Text,
			&task.IdLang,
		)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found: " + err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
		}
		return
	}

	c.HTML(http.StatusOK, "task_solution.html", gin.H{
		"task": task,
	})
}

func SaveSolution(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req SolutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	_, err := postgresDb.Exec(`
        INSERT INTO solution (id_task, id_user, code)
        VALUES ($1, $2, $3)
        ON CONFLICT (id_task, id_user) 
        DO UPDATE SET code = EXCLUDED.code`,
		req.TaskID, userID, req.Code)

	if err != nil {
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to save solution",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Solution saved successfully"})
}

func getSolution(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	taskID, err := strconv.Atoi(c.Param("task_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}
	var code string
	err = postgresDb.QueryRow(`
		SELECT code FROM solution 
		WHERE id_task = $1 AND id_user = $2`,
		taskID, userID.(int)).Scan(&code)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Solution not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get solution"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": code})
}

func Logout(c *gin.Context) {
	c.SetCookie("jwtToken", "", -1, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"message": "Successfully logged out",
	})
}

func main() {
	if err := initDb(); err != nil {
		log.Fatalf("PostgreSQL init error: %v", err)
	}
	defer postgresDb.Close()

	router := gin.Default()
	router.LoadHTMLGlob("./templates/*")

	// Обработчик для 404 ошибок
	router.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusNotFound, "404.html", gin.H{
			"requestedPath": c.Request.URL.Path,
		})
	})

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "auth.html", gin.H{})
	})

	router.GET("/profile", authMiddleware(), func(c *gin.Context) {
		userID, existId := c.Get("user_id")
		userName, existName := c.Get("user_name")
		userEmail, existEmail := c.Get("user_email")
		if !existId || !existName || !existEmail {
			return
		}
		c.HTML(http.StatusOK, "profile.html", gin.H{
			"userId":    userID,
			"userName":  userName,
			"userEmail": userEmail,
		})
	})

	router.GET("/task", authMiddleware(), getAllTask)
	router.GET("/task/:id", authMiddleware(), getTask)

	router.POST("/register", registerUser) // Убрал authMiddleware, так как регистрация должна быть доступна без авторизации
	router.POST("/login", loginUser)       // Убрал authMiddleware для логина
	router.POST("/logout", authMiddleware(), Logout)

	authGroup := router.Group("/")
	authGroup.Use(authMiddleware())
	{
		authGroup.POST("/solution", SaveSolution)
		authGroup.GET("/solution/:task_id", getSolution)
	}

	if err := router.Run(":8081"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
