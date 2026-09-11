package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"github.com/parewaBhuvan/nexlink/internal/auth"
	"github.com/parewaBhuvan/nexlink/internal/otp"
	"github.com/parewaBhuvan/nexlink/internal/router"
	"github.com/parewaBhuvan/nexlink/internal/ws"
)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, relying on real environment variables")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("unable to create connection pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("unable to ping database: %v", err)
	}
	log.Println("connected to Postgres successfully")

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		log.Fatal("REDIS_ADDR is not set")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("unable to ping redis: %v", err)
	}
	log.Println("connected to Redis successfully")

	otpSender := otp.NewConsoleSender()
	authService := auth.NewService(pool, rdb, otpSender)
	hub := ws.NewHub()
	go hub.Run()

	wsService := ws.NewService(pool)

	r := router.NewRouter(authService , hub, wsService);

	r.GET("/health", func(c *gin.Context) {
		dbErr := pool.Ping(c.Request.Context())
		redisErr := rdb.Ping(c.Request.Context()).Err()

		if dbErr != nil || redisErr != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "error",
				"db":     dbErr == nil,
				"redis":  redisErr == nil,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"db":     "connected",
			"redis":  "connected",
		})
	})

	r.Run(":8080")
}
