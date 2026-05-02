package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/auditrail/auditrail/internal/application"
)

// Context keys.
const (
	ctxKeyApp = "auditrail_app"
)

// Middleware builds a Fiber handler that validates API Key + HMAC.
type Middleware struct {
	Repo     *application.Repository
	Redis    *redis.Client
	MaxSkew  time.Duration
	NonceTTL time.Duration
}

// Required produces a middleware that authenticates every request.
func (m *Middleware) Required() fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiKey := extractAPIKey(c)
		if apiKey == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing_api_key"})
		}

		app, err := m.Repo.GetByAPIKey(c.UserContext(), apiKey)
		if err != nil {
			if errors.Is(err, application.ErrNotFound) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid_api_key"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "auth_lookup_failed"})
		}

		tsHeader := c.Get("X-Timestamp")
		sigHeader := c.Get("X-Signature")
		nonce := c.Get("X-Nonce")

		if tsHeader == "" || sigHeader == "" || nonce == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing_signature_headers"})
		}

		ts, err := time.Parse(time.RFC3339, tsHeader)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_timestamp"})
		}
		now := time.Now().UTC()
		if ts.After(now.Add(m.MaxSkew)) || ts.Before(now.Add(-m.MaxSkew)) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "timestamp_out_of_window"})
		}

		body := c.Body()
		expected := Sign(app.SecretKey, tsHeader, nonce, body)
		if !constantTimeEqual(sigHeader, expected) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid_signature"})
		}

		// Nonce single-use (replay prevention).
		if m.Redis != nil {
			key := "nonce:" + app.ID + ":" + nonce
			ok, err := m.Redis.SetNX(c.UserContext(), key, "1", m.NonceTTL).Result()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "nonce_store_failed"})
			}
			if !ok {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "nonce_reused"})
			}
		}

		c.Locals(ctxKeyApp, app)
		return c.Next()
	}
}

// FromContext retrieves the authenticated application from the Fiber context.
func FromContext(c *fiber.Ctx) *application.Application {
	v := c.Locals(ctxKeyApp)
	if v == nil {
		return nil
	}
	return v.(*application.Application)
}

// FromGoContext retrieves application stored in a Go context (for worker code).
func FromGoContext(ctx context.Context) *application.Application {
	v := ctx.Value(ctxKeyApp)
	if v == nil {
		return nil
	}
	return v.(*application.Application)
}

// Sign generates the expected signature string ("hmac-sha256=<hex>").
func Sign(secret, timestamp, nonce string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("\n"))
	mac.Write([]byte(nonce))
	mac.Write([]byte("\n"))
	mac.Write(body)
	return "hmac-sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func extractAPIKey(c *fiber.Ctx) string {
	h := c.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	// Also accept X-Api-Key for simple clients.
	return c.Get("X-Api-Key")
}

func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return hmac.Equal([]byte(a), []byte(b))
}
