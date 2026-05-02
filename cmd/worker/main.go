package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/auditrail/auditrail/internal/config"
	"github.com/auditrail/auditrail/internal/event"
	httpsrv "github.com/auditrail/auditrail/internal/http"
	"github.com/auditrail/auditrail/internal/logging"
	"github.com/auditrail/auditrail/internal/queue"
	pgstore "github.com/auditrail/auditrail/internal/storage/postgres"
)

// Worker consumes audit.* and error.* subjects from NATS and persists to PostgreSQL.
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

	q, err := queue.Connect(ctx, cfg.NATS.URL, cfg.NATS.StreamName)
	if err != nil {
		log.Fatal().Err(err).Msg("nats connect")
	}
	defer q.Close()

	repo := event.NewPostgresRepo(pool)

	cons, err := q.Stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:        "worker-audit-error",
		FilterSubjects: []string{"events.audit.>", "events.error.>"},
		AckPolicy:      jetstream.AckExplicitPolicy,
		MaxDeliver:     5,
		AckWait:        30 * time.Second,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("consumer create")
	}

	ccCtx, ccCancel := context.WithCancel(context.Background())
	defer ccCancel()

	_, err = cons.Consume(func(msg jetstream.Msg) {
		var e event.Event
		if err := json.Unmarshal(msg.Data(), &e); err != nil {
			log.Error().Err(err).Msg("bad event payload, terminating")
			_ = msg.Term()
			return
		}
		switch e.Category {
		case event.CategoryAudit:
			if err := repo.InsertAudit(ccCtx, &e); err != nil {
				log.Error().Err(err).Str("event_id", e.EventID).Msg("insert audit failed")
				_ = msg.Nak()
				return
			}
		case event.CategoryError:
			fp := httpsrv.Fingerprint(&e)
			if err := repo.InsertError(ccCtx, &e, fp); err != nil {
				log.Error().Err(err).Str("event_id", e.EventID).Msg("insert error failed")
				_ = msg.Nak()
				return
			}
		default:
			// Not our responsibility; ack so JetStream doesn't redeliver.
		}
		_ = msg.Ack()
	})
	if err != nil {
		log.Fatal().Err(err).Msg("consume failed")
	}

	log.Info().Msg("worker started (audit + error)")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Info().Msg("worker shutting down")
}
