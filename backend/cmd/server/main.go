package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/topic-voting/backend/internal/config"
	"github.com/topic-voting/backend/internal/database"
	"github.com/topic-voting/backend/internal/handler"
	"github.com/topic-voting/backend/internal/repository"
	"github.com/topic-voting/backend/internal/service"
	"github.com/topic-voting/backend/migrations"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := database.New(ctx, cfg.DSN())
	cancel()
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer db.Close()

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	migrationsFS, err := fs.Sub(migrations.FS, ".")
	if err != nil {
		cancel()
		log.Fatalf("prepare migrations fs: %v", err)
	}
	if err := db.RunMigrations(ctx, migrationsFS); err != nil {
		cancel()
		log.Fatalf("run migrations: %v", err)
	}
	cancel()

	topicRepo := repository.NewTopicRepository(db.Pool)

	topicSvc := service.NewTopicService(topicRepo)
	topicHandler := handler.NewTopicHandler(topicSvc)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSAllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/api/health", func(c *gin.Context) {
		if err := db.Health(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	api := r.Group("/api")
	topicHandler.RegisterRoutes(api)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.ServerPort),
		Handler: r,
	}

	go func() {
		log.Printf("server starting on :%s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown: %v", err)
	}

	log.Println("closing database connection...")
	db.Close()
	log.Println("server stopped")
}