package auth

import "github.com/gofiber/fiber/v2"

// MasterTokenRequired protects admin routes with a single shared master token.
func MasterTokenRequired(token string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		provided := c.Get("X-Admin-Token")
		if token == "" || provided != token {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "admin_unauthorized"})
		}
		return c.Next()
	}
}
