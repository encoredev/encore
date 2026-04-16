package handler

import (
	"github.com/gofiber/fiber/v3"
)

var version = "dev"

func Health(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"version": version,
	})
}
