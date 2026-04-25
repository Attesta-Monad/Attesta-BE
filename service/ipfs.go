package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/GPadaka19/attesta-be/config"
	"github.com/GPadaka19/attesta-be/model"
)

const (
	w3sUploadURL = "https://api.web3.storage/upload"
	pinataURL    = "https://api.pinata.cloud/pinning/pinJSONToIPFS"
	ipfsTimeout  = 30 * time.Second
)

var ipfsClient = &http.Client{Timeout: ipfsTimeout}

type evidenceMeta struct {
	Owner           string `json:"owner"`
	Repo            string `json:"repo"`
	Author          string `json:"author"`
	CommitsFetched  int    `json:"commits_fetched"`
	CommitsAnalyzed int    `json:"commits_analyzed"`
}

type evidencePackage struct {
	GeneratedAt string            `json:"generated_at"`
	Meta        evidenceMeta      `json:"meta"`
	SkillProof  *model.SkillProof `json:"skill_proof"`
	Commits     []model.AICommit  `json:"commits"`
}

// UploadEvidence uploads the evidence package to IPFS and returns the CID.
// Priority: Pinata (PINATA_JWT) → Legacy Web3.Storage (W3S_TOKEN)
func UploadEvidence(owner, repo, author string, commitsFetched int, payload *model.GitHubPayload, proof *model.SkillProof) (string, error) {
	pkg := evidencePackage{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Meta: evidenceMeta{
			Owner:           owner,
			Repo:            repo,
			Author:          author,
			CommitsFetched:  commitsFetched,
			CommitsAnalyzed: len(payload.Commits),
		},
		SkillProof: proof,
		Commits:    payload.Commits,
	}

	body, err := json.Marshal(pkg)
	if err != nil {
		return "", fmt.Errorf("ipfs: marshal evidence: %w", err)
	}

	log.Printf("[IPFS] uploading evidence bytes=%d", len(body))

	if config.App.PinataJWT != "" {
		return uploadViaPinata(body)
	}
	if config.App.W3SToken != "" {
		return uploadViaW3S(body)
	}

	return "", fmt.Errorf("IPFS_UPLOAD_FAILED: set PINATA_JWT or W3S_TOKEN in environment")
}

// --- Pinata ---

type pinataRequest struct {
	PinataContent json.RawMessage `json:"pinataContent"`
}

type pinataResponse struct {
	IpfsHash string `json:"IpfsHash"`
}

func uploadViaPinata(body []byte) (string, error) {
	reqBody, err := json.Marshal(pinataRequest{PinataContent: json.RawMessage(body)})
	if err != nil {
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}

	req, err := http.NewRequest("POST", pinataURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.App.PinataJWT)

	resp, err := ipfsClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[IPFS] pinata upload failed status=%d body=%s", resp.StatusCode, string(raw))
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}

	var out pinataResponse
	if err := json.Unmarshal(raw, &out); err != nil || out.IpfsHash == "" {
		log.Printf("[IPFS] pinata unexpected response: %s", string(raw))
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}

	log.Printf("[IPFS] upload success (pinata) cid=%s", out.IpfsHash)
	return out.IpfsHash, nil
}

// --- Legacy Web3.Storage ---

type w3sUploadResponse struct {
	CID string `json:"cid"`
}

func uploadViaW3S(body []byte) (string, error) {
	req, err := http.NewRequest("POST", w3sUploadURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}
	req.Header.Set("Authorization", "Bearer "+config.App.W3SToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ipfsClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[IPFS] w3s upload failed status=%d body=%s", resp.StatusCode, string(raw))
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}

	var out w3sUploadResponse
	if err := json.Unmarshal(raw, &out); err != nil || out.CID == "" {
		log.Printf("[IPFS] w3s unexpected response: %s", string(raw))
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}

	log.Printf("[IPFS] upload success (w3s) cid=%s", out.CID)
	return out.CID, nil
}
