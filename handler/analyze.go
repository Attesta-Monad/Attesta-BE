package handler

import (
	"github.com/GPadaka19/attesta-be/service"
	"github.com/gofiber/fiber/v2"
)

type analyzeRequest struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Author string `json:"author"`
}

func Analyze(c *fiber.Ctx) error {
	var req analyzeRequest
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

	proof, err := service.AnalyzeCommits(payload)
	if err != nil {
		return handleServiceError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"meta": fiber.Map{
			"owner":            req.Owner,
			"repo":             req.Repo,
			"author":           req.Author,
			"commits_fetched":  fetched,
			"commits_analyzed": len(payload.Commits),
		},
		"skill_proof": proof,
	})
}
