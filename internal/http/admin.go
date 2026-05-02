package http

import (
	"github.com/gofiber/fiber/v2"

	"github.com/auditrail/auditrail/internal/application"
)

// AdminHandler handles application (tenant) management endpoints.
type AdminHandler struct {
	Repo *application.Repository
}

type createAppReq struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Create handles POST /v1/admin/applications.
func (h *AdminHandler) Create(c *fiber.Ctx) error {
	var req createAppReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_body"})
	}
	if req.Code == "" || req.Type == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "code_and_type_required"})
	}
	app, err := h.Repo.Create(c.UserContext(), req.Code, req.Name, req.Type)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "create_failed", "detail": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(app)
}

// List handles GET /v1/admin/applications.
func (h *AdminHandler) List(c *fiber.Ctx) error {
	apps, err := h.Repo.List(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "list_failed"})
	}
	return c.JSON(fiber.Map{"items": apps, "count": len(apps)})
}

// Rotate handles POST /v1/admin/applications/:id/rotate-key.
func (h *AdminHandler) Rotate(c *fiber.Ctx) error {
	id := c.Params("id")
	app, err := h.Repo.RotateKey(c.UserContext(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not_found"})
	}
	return c.JSON(app)
}

// Revoke handles DELETE /v1/admin/applications/:id.
func (h *AdminHandler) Revoke(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.Repo.Revoke(c.UserContext(), id); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not_found"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
