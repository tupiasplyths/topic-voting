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
	"github.com/google/uuid"
	"nhooyr.io/websocket"

	"github.com/topic-voting/backend/internal/config"
	"github.com/topic-voting/backend/internal/database"
	"github.com/topic-voting/backend/internal/handler"
	"github.com/topic-voting/backend/internal/model"
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
	voteRepo := repository.NewVoteRepository(db.Pool)

	tallyCache, err := service.NewVoteTallyCache(context.Background(), voteRepo, cfg.DBFlushInterval, topicRepo)
	if err != nil {
		log.Fatalf("create tally cache: %v", err)
	}
	tallyCache.Start()

	classifierClient := service.NewClassifierClient(cfg.ClassifierURL, cfg.ClassifierTimeout)

	for i := 0; i < 5; i++ {
		healthCtx, healthCancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := classifierClient.HealthCheck(healthCtx)
		healthCancel()
		if err == nil {
			log.Println("classifier service is healthy")
			break
		}
		if i == 4 {
			log.Printf("warning: classifier service unavailable: %v", err)
		} else {
			log.Printf("classifier health check failed (attempt %d/5): %v", i+1, err)
			time.Sleep(1 * time.Second)
		}
	}

	getLB := func(topicID uuid.UUID) (*model.Leaderboard, error) {
		return tallyCache.GetLeaderboard(topicID)
	}

	wsHub := handler.NewWebSocketHub(getLB)
	go wsHub.Run()

	voteProcessor := service.NewVoteProcessor(
		cfg.VoteQueueCapacity,
		cfg.ClassifierWorkers,
		classifierClient,
		tallyCache,
		wsHub,
	)
	voteProcessor.Start()

	topicSvc := service.NewTopicService(topicRepo)
	voteSvc := service.NewVoteService(topicRepo, voteProcessor, tallyCache, cfg.ClassifierThreshold)

	topicHandler := handler.NewTopicHandler(topicSvc)
	voteHandler := handler.NewVoteHandler(voteSvc)

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
	voteHandler.RegisterRoutes(api)

	r.GET("/ws/dashboard", func(c *gin.Context) {
		topicIDStr := c.Query("topic_id")
		if topicIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "topic_id is required"})
			return
		}
		topicID, err := uuid.Parse(topicIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid topic_id"})
			return
		}

		ws, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
			OriginPatterns: cfg.CORSAllowedOrigins,
		})
		if err != nil {
			return
		}
		wsHub.HandleDashboard(ws, topicID, cfg.WSPingInterval, cfg.WSPongTimeout)
	})

	r.GET("/ws/chat", func(c *gin.Context) {
		topicIDStr := c.Query("topic_id")
		if topicIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "topic_id is required"})
			return
		}
		topicID, err := uuid.Parse(topicIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid topic_id"})
			return
		}

		ws, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
			OriginPatterns: cfg.CORSAllowedOrigins,
		})
		if err != nil {
			return
		}
		wsHub.HandleChat(ws, topicID, cfg.WSPingInterval, cfg.WSPongTimeout)
	})

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

	wsHub.Stop()
	voteProcessor.Stop()
	tallyCache.Stop()

	log.Println("closing database connection...")
	db.Close()
	log.Println("server stopped")
}
