package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/auditrail/auditrail/internal/application"
	"github.com/auditrail/auditrail/internal/config"
	"github.com/auditrail/auditrail/internal/event"
	httpsrv "github.com/auditrail/auditrail/internal/http"
	"github.com/auditrail/auditrail/internal/logging"
	"github.com/auditrail/auditrail/internal/queue"
	pgstore "github.com/auditrail/auditrail/internal/storage/postgres"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logging.New(cfg.LogLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgstore.New(ctx, cfg.Postgres.URL, cfg.Postgres.MaxConns, cfg.Postgres.MaxConnLifetime)
	if err != nil {
		log.Fatal().Err(err).Msg("postgres connect")
	}
	defer pool.Close()

	if err := pgstore.Migrate(ctx, pool, "migrations/postgres"); err != nil {
		log.Fatal().Err(err).Msg("postgres migrate")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Warn().Err(err).Msg("redis ping failed (continuing, rate limit & nonce disabled)")
	}

	q, err := queue.Connect(ctx, cfg.NATS.URL, cfg.NATS.StreamName)
	if err != nil {
		log.Fatal().Err(err).Msg("nats connect")
	}
	defer q.Close()

	appRepo := application.NewRepository(pool)
	eventRepo := event.NewPostgresRepo(pool)

	srv := httpsrv.NewServer(cfg, log, appRepo, eventRepo, q, rdb)

	go func() {
		log.Info().Str("addr", cfg.HTTP.Addr).Msg("API listening")
		if err := srv.Listen(); err != nil {
			log.Fatal().Err(err).Msg("server stopped")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Info().Msg("shutting down...")
	shutdownCtx, cancelShut := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShut()
	_ = shutdownCtx
	_ = srv.Shutdown()
}
