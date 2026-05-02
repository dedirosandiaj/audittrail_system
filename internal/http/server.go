package http

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/auditrail/auditrail/internal/application"
	"github.com/auditrail/auditrail/internal/auth"
	"github.com/auditrail/auditrail/internal/config"
	"github.com/auditrail/auditrail/internal/event"
	"github.com/auditrail/auditrail/internal/queue"
	"github.com/auditrail/auditrail/internal/ratelimit"
	"github.com/redis/go-redis/v9"
)

// Server holds dependencies for the HTTP API.
type Server struct {
	App       *fiber.App
	Cfg       *config.Config
	Log       zerolog.Logger
	AppRepo   *application.Repository
	EventRepo *event.PostgresRepo
	Queue     *queue.Client
	Redis     *redis.Client
}

// NewServer wires routes and middlewares.
func NewServer(cfg *config.Config, log zerolog.Logger, appRepo *application.Repository,
	eventRepo *event.PostgresRepo, q *queue.Client, rdb *redis.Client) *Server {

	app := fiber.New(fiber.Config{
		BodyLimit:             cfg.HTTP.BodyLimitBytes,
		ReadTimeout:           cfg.HTTP.ReadTimeout,
		WriteTimeout:          cfg.HTTP.WriteTimeout,
		DisableStartupMessage: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			log.Error().Err(err).Str("path", c.Path()).Msg("unhandled error")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal"})
		},
	})

	// ---- system routes ----
	app.Get("/healthz", func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/readyz", func(c *fiber.Ctx) error {
		if q != nil && q.Conn != nil && !q.Conn.IsConnected() {
			return c.Status(fiber.StatusServiceUnavailable).SendString("nats_down")
		}
		return c.SendString("ready")
	})

	// ---- auth middlewares ----
	authMW := &auth.Middleware{
		Repo:     appRepo,
		Redis:    rdb,
		MaxSkew:  cfg.Security.MaxClockSkew,
		NonceTTL: cfg.Security.NonceTTL,
	}
	adminMW := auth.MasterTokenRequired(cfg.Security.MasterToken)

	// ---- handlers ----
	limiter := ratelimit.New(rdb)
	ingest := &IngestHandler{Queue: q, Limiter: limiter, Cfg: cfg, Log: log}
	query := &QueryHandler{Repo: eventRepo}
	admin := &AdminHandler{Repo: appRepo}

	v1 := app.Group("/v1")

	// Ingestion (authenticated via API key + HMAC)
	ev := v1.Group("/events", authMW.Required())
	ev.Post("/", ingest.Single)
	ev.Post("/bulk", ingest.Bulk)

	// Query (authenticated via API key + HMAC; scoped to caller's application)
	v1.Get("/events", authMW.Required(), func(c *fiber.Ctx) error {
		if a := auth.FromContext(c); a != nil && c.Query("application_id") == "" {
			c.Request().URI().QueryArgs().Add("application_id", a.ID)
		}
		return query.List(c)
	})

	// Admin (master token)
	adm := v1.Group("/admin", adminMW)
	adm.Post("/applications", admin.Create)
	adm.Get("/applications", admin.List)
	adm.Post("/applications/:id/rotate-key", admin.Rotate)
	adm.Delete("/applications/:id", admin.Revoke)

	s := &Server{
		App: app, Cfg: cfg, Log: log,
		AppRepo: appRepo, EventRepo: eventRepo, Queue: q, Redis: rdb,
	}
	return s
}

// Listen starts the HTTP server (blocking).
func (s *Server) Listen() error { return s.App.Listen(s.Cfg.HTTP.Addr) }

// Shutdown gracefully stops the server.
func (s *Server) Shutdown() error {
	return s.App.ShutdownWithTimeout(s.Cfg.HTTP.ShutdownTimeout)
}
