package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func errorResponse(c *fiber.Ctx, status int, message, code string) error {
	return c.Status(status).JSON(fiber.Map{
		"success": false,
		"error":   message,
		"code":    code,
	})
}

// handleServiceError maps well-known error sentinel strings to HTTP responses.
func handleServiceError(c *fiber.Ctx, err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "GITHUB_USER_NOT_FOUND"):
		return errorResponse(c, fiber.StatusNotFound, "author has no commits in this repo", "GITHUB_USER_NOT_FOUND")
	case strings.Contains(msg, "GITHUB_REPO_NOT_FOUND"):
		return errorResponse(c, fiber.StatusNotFound, "repo not found or is private", "GITHUB_REPO_NOT_FOUND")
	case strings.Contains(msg, "GITHUB_RATE_LIMIT"):
		return errorResponse(c, fiber.StatusTooManyRequests, "GitHub rate limit reached", "GITHUB_RATE_LIMIT")
	case strings.Contains(msg, "NO_VALID_COMMITS"):
		return errorResponse(c, fiber.StatusUnprocessableEntity, "no valid commits found (all were merge commits or oversized)", "NO_VALID_COMMITS")
	case strings.Contains(msg, "AI_PARSE_ERROR"):
		return errorResponse(c, fiber.StatusInternalServerError, "AI returned invalid JSON", "AI_PARSE_ERROR")
	case strings.Contains(msg, "IPFS_UPLOAD_FAILED"):
		return errorResponse(c, fiber.StatusBadGateway, "failed to upload evidence to IPFS", "IPFS_UPLOAD_FAILED")
	case strings.Contains(msg, "CONTRACT_NOT_SET"):
		return errorResponse(c, fiber.StatusInternalServerError, "CONTRACT_ADDRESS is not set", "CONTRACT_NOT_SET")
	case strings.Contains(msg, "TX_FAILED"):
		return errorResponse(c, fiber.StatusBadGateway, "blockchain transaction failed", "TX_FAILED")
	default:
		return errorResponse(c, fiber.StatusInternalServerError, msg, "INTERNAL_ERROR")
	}
}
