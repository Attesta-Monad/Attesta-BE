package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/GPadaka19/attesta-be/config"
	"github.com/GPadaka19/attesta-be/model"
)

const (
	w3sUploadURL = "https://api.web3.storage/upload"
	ipfsTimeout  = 30 * time.Second
)

var ipfsClient = &http.Client{Timeout: ipfsTimeout}

var cidRegex = regexp.MustCompile(`\b(bafy[0-9a-z]{20,}|Qm[1-9A-HJ-NP-Za-km-z]{44})\b`)

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

type w3sUploadResponse struct {
	CID string `json:"cid"`
}

// UploadEvidence uploads the evidence package to Web3.Storage and returns CID.
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

	// Prefer legacy Web3.Storage API token if provided.
	if config.App.W3SToken != "" {
		cid, err := uploadViaLegacyW3S(body)
		// If legacy endpoint is flaky/maintenance, fallback to CLI (Storacha/w3up).
		if err == nil {
			return cid, nil
		}
		if strings.Contains(err.Error(), "IPFS_LEGACY_MAINTENANCE") {
			log.Printf("[IPFS] legacy maintenance detected, falling back to w3 cli")
			return uploadViaW3CLI(body)
		}
		return "", err
	}

	// Fallback: Storacha/w3up setup via CLI (`w3 up <file>`).
	// This assumes `w3 login` + `w3 space use <space>` have been done on the machine.
	return uploadViaW3CLI(body)
}

func uploadViaLegacyW3S(body []byte) (string, error) {
	req, err := http.NewRequest("POST", w3sUploadURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ipfs: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+config.App.W3SToken)
	req.Header.Set("Content-Type", "application/json")

	log.Printf("[IPFS] uploading evidence bytes=%d", len(body))
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
		// Observed: 503 ERROR_MAINTENANCE even when status page is operational.
		if resp.StatusCode == 503 && bytes.Contains(raw, []byte("ERROR_MAINTENANCE")) {
			log.Printf("[IPFS] legacy maintenance status=503 body=%s", string(raw))
			return "", fmt.Errorf("IPFS_LEGACY_MAINTENANCE")
		}
		log.Printf("[IPFS] upload failed status=%d body=%s", resp.StatusCode, string(raw))
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}

	var out w3sUploadResponse
	if err := json.Unmarshal(raw, &out); err != nil || out.CID == "" {
		log.Printf("[IPFS] unexpected response: %s", string(raw))
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}

	log.Printf("[IPFS] upload success cid=%s", out.CID)
	return out.CID, nil
}

func uploadViaW3CLI(body []byte) (string, error) {
	f, err := os.CreateTemp("", "attesta-evidence-*.json")
	if err != nil {
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}
	tmpPath := f.Name()
	defer os.Remove(tmpPath)

	if _, err := f.Write(body); err != nil {
		f.Close()
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}

	ctx, cancel := context.WithTimeout(context.Background(), ipfsTimeout)
	defer cancel()

	// `w3 up` prints a gateway URL containing the CID.
	cmd := exec.CommandContext(ctx, "w3", "up", tmpPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[IPFS] w3 up failed: %v output=%s", err, string(out))
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}

	s := string(out)
	m := cidRegex.FindStringSubmatch(s)
	if len(m) == 0 {
		log.Printf("[IPFS] w3 up output missing cid: %s", s)
		return "", fmt.Errorf("IPFS_UPLOAD_FAILED")
	}

	cid := m[1]
	log.Printf("[IPFS] upload success (w3) cid=%s", cid)
	return cid, nil
}
