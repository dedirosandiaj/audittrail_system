package http

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/auditrail/auditrail/internal/event"
)

// QueryHandler serves read-only endpoints for audit events.
type QueryHandler struct {
	Repo *event.PostgresRepo
}

// List handles GET /v1/events.
func (h *QueryHandler) List(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	afterID, _ := strconv.ParseInt(c.Query("after_id", "0"), 10, 64)

	f := event.QueryFilter{
		ApplicationID: c.Query("application_id"),
		ActorID:       c.Query("actor_id"),
		Action:        c.Query("action"),
		ResourceType:  c.Query("resource_type"),
		ResourceID:    c.Query("resource_id"),
		Status:        c.Query("status"),
		FromTime:      c.Query("from"),
		ToTime:        c.Query("to"),
		AfterID:       afterID,
		Limit:         limit,
	}
	items, err := h.Repo.ListAudit(c.UserContext(), f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "query_failed", "detail": err.Error()})
	}
	return c.JSON(fiber.Map{
		"count": len(items),
		"items": items,
	})
}
