package main

import (
	"log"

	"github.com/GPadaka19/attesta-be/config"
	"github.com/GPadaka19/attesta-be/handler"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	config.Load()

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
				"code":    "INTERNAL_ERROR",
			})
		},
	})

	app.Use(logger.New())

	app.Get("/health", handler.Health)
	app.Post("/analyze", handler.Analyze)
	app.Post("/debug/github", handler.DebugGitHub)

	addr := ":" + config.App.Port
	log.Printf("server starting on %s", addr)
	log.Fatal(app.Listen(addr))
}
