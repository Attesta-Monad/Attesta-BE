package handler

import (
	"fmt"

	"github.com/GPadaka19/attesta-be/service"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gofiber/fiber/v2"
)

type attestRequest struct {
	Owner            string `json:"owner"`
	Repo             string `json:"repo"`
	Author           string `json:"author"`
	RecipientAddress string `json:"recipient_address"`
}

func Attest(c *fiber.Ctx) error {
	var req attestRequest
	if err := c.BodyParser(&req); err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid request body", "INVALID_INPUT")
	}
	if req.Owner == "" || req.Repo == "" || req.Author == "" || req.RecipientAddress == "" {
		return errorResponse(c, fiber.StatusBadRequest, "owner, repo, author, and recipient_address are required", "INVALID_INPUT")
	}
	if !common.IsHexAddress(req.RecipientAddress) {
		return errorResponse(c, fiber.StatusBadRequest, "recipient_address is invalid", "INVALID_INPUT")
	}
	recipient := common.HexToAddress(req.RecipientAddress)

	payload, fetched, err := service.FetchGitHubData(req.Owner, req.Repo, req.Author)
	if err != nil {
		return handleServiceError(c, err)
	}

	proof, err := service.AnalyzeCommits(payload)
	if err != nil {
		return handleServiceError(c, err)
	}

	cid, err := service.UploadEvidence(req.Owner, req.Repo, req.Author, fetched, payload, proof)
	if err != nil {
		return handleServiceError(c, err)
	}

	res, err := service.SubmitAttestation(req.Owner, req.Repo, req.Author, recipient, proof, len(payload.Commits), cid)
	if err != nil {
		return handleServiceError(c, err)
	}

	return c.JSON(fiber.Map{
		"success":        true,
		"attestation_id": res.AttestationID,
		"tx_hash":        res.TxHash,
		"explorer_url":   res.ExplorerURL,
		"evidence_cid":   cid,
		"evidence_url":   fmt.Sprintf("https://%s.ipfs.w3s.link", cid),
		"skill_proof":    proof,
	})
}

