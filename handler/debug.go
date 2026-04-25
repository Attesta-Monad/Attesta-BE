package handler

import (
	"github.com/GPadaka19/attesta-be/service"
	"github.com/gofiber/fiber/v2"
)

type debugGitHubRequest struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Author string `json:"author"`
}

func DebugGitHub(c *fiber.Ctx) error {
	var req debugGitHubRequest
	if err := c.BodyParser(&req); err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid request body", "INVALID_INPUT")
	}
	if req.Owner == "" || req.Repo == "" || req.Author == "" {
		return errorResponse(c, fiber.StatusBadRequest, "owner, repo, and author are required", "INVALID_INPUT")
	}

	payload, fetched, err := service.FetchGitHubData(req.Owner, req.Repo, req.Author)
	if err != nil {
		return handleServiceError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"meta": fiber.Map{
			"commits_fetched":  fetched,
			"commits_analyzed": len(payload.Commits),
		},
		"payload": payload,
	})
}
