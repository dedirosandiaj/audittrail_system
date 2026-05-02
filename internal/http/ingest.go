package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/auditrail/auditrail/internal/auth"
	"github.com/auditrail/auditrail/internal/config"
	"github.com/auditrail/auditrail/internal/event"
	"github.com/auditrail/auditrail/internal/queue"
	"github.com/auditrail/auditrail/internal/ratelimit"
)

// IngestHandler handles POST /v1/events and /v1/events/bulk.
type IngestHandler struct {
	Queue   *queue.Client
	Limiter *ratelimit.Limiter
	Cfg     *config.Config
	Log     zerolog.Logger
}

type ingestResponse struct {
	Accepted bool     `json:"accepted"`
	EventIDs []string `json:"event_ids"`
	Count    int      `json:"count"`
}

// Single handles POST /v1/events.
func (h *IngestHandler) Single(c *fiber.Ctx) error {
	app := auth.FromContext(c)
	if app == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthenticated"})
	}

	var e event.Event
	if err := json.Unmarshal(c.Body(), &e); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_json", "detail": err.Error()})
	}
	if err := e.Validate(h.Cfg.Security.MaxClockSkew); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_event", "detail": err.Error()})
	}
	e.ApplicationID = app.ID
	e.ApplicationCode = app.Code

	if !h.allow(c, app.ID, e.Category) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "rate_limited"})
	}

	if err := h.publish(c, &e); err != nil {
		h.Log.Error().Err(err).Msg("publish failed")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "publish_failed"})
	}

	return c.Status(fiber.StatusAccepted).JSON(ingestResponse{
		Accepted: true,
		EventIDs: []string{e.EventID},
		Count:    1,
	})
}

// Bulk handles POST /v1/events/bulk (max 500 events).
func (h *IngestHandler) Bulk(c *fiber.Ctx) error {
	app := auth.FromContext(c)
	if app == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthenticated"})
	}

	var events []event.Event
	if err := json.Unmarshal(c.Body(), &events); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_json", "detail": err.Error()})
	}
	if len(events) == 0 || len(events) > 500 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bulk_size_out_of_range"})
	}

	ids := make([]string, 0, len(events))
	for i := range events {
		if err := events[i].Validate(h.Cfg.Security.MaxClockSkew); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":  "invalid_event",
				"index":  i,
				"detail": err.Error(),
			})
		}
		events[i].ApplicationID = app.ID
		events[i].ApplicationCode = app.Code

		if !h.allow(c, app.ID, events[i].Category) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "rate_limited"})
		}
		if err := h.publish(c, &events[i]); err != nil {
			h.Log.Error().Err(err).Msg("publish failed")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "publish_failed"})
		}
		ids = append(ids, events[i].EventID)
	}

	return c.Status(fiber.StatusAccepted).JSON(ingestResponse{
		Accepted: true,
		EventIDs: ids,
		Count:    len(ids),
	})
}

func (h *IngestHandler) publish(c *fiber.Ctx, e *event.Event) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return h.Queue.Publish(c.UserContext(), e.Subject(), data)
}

func (h *IngestHandler) allow(c *fiber.Ctx, appID string, cat event.Category) bool {
	limit := h.Cfg.RateLimit.AuditPerMinute
	switch cat {
	case event.CategoryError:
		limit = h.Cfg.RateLimit.ErrorPerMinute
	case event.CategoryLog:
		limit = h.Cfg.RateLimit.LogPerMinute
	case event.CategoryMetric, event.CategoryBehavior:
		limit = h.Cfg.RateLimit.MetricPerMinute
	}
	ok, remaining, err := h.Limiter.Allow(c.UserContext(), appID, string(cat), limit)
	if err != nil {
		h.Log.Warn().Err(err).Msg("rate limiter failed open")
		return true
	}
	c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
	c.Set("X-RateLimit-Limit", strconv.Itoa(limit))
	return ok
}

// Fingerprint computes a stable signature for an error event
// (used for grouping similar errors).
func Fingerprint(e *event.Event) string {
	h := sha256.New()
	h.Write([]byte(e.ApplicationID))
	h.Write([]byte("|"))
	h.Write([]byte(e.Action))
	h.Write([]byte("|"))
	h.Write(e.Payload)
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// NowRFC3339 returns the current time formatted for HTTP headers.
func NowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }
