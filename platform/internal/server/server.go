package server

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/rs/zerolog"

	"encoru.dev/platform/internal/handler"
)

func New(log *zerolog.Logger) *fiber.App {
	app := fiber.New()

	app.Use(requestid.New())
	app.Use(zerologMiddleware(log))
	app.Use(recover.New())
	app.Use(cors.New())

	app.Get("/health", handler.Health)

	return app
}
