package main

import (
	"log"

	"github.com/GPadaka19/attesta-be/config"
	"github.com/GPadaka19/attesta-be/handler"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
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

	// CORS — izinkan frontend Vercel mengakses API
	app.Use(cors.New(cors.Config{
		AllowOrigins: "https://attesta-fe.vercel.app",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	app.Use(logger.New())

	app.Get("/health", handler.Health)
	app.Post("/analyze", handler.Analyze)
	app.Post("/attest", handler.Attest)
	app.Post("/debug/github", handler.DebugGitHub)

	addr := ":" + config.App.Port
	log.Printf("server starting on %s", addr)
	log.Fatal(app.Listen(addr))
}
