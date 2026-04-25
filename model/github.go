package model

import "time"

type RepoLanguages map[string]int

type CommitSummary struct {
	SHA     string    `json:"sha"`
	Message string    `json:"message"`
	Date    time.Time `json:"date"`
	Parents []Parent  `json:"parents"`
}

type Parent struct {
	SHA string `json:"sha"`
}

type CommitDetail struct {
	SHA   string `json:"sha"`
	Stats Stats  `json:"stats"`
	Files []FileChange `json:"files"`
}

type Stats struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	Total     int `json:"total"`
}

type FileChange struct {
	Filename     string `json:"filename"`
	Status       string `json:"status"`
	Additions    int    `json:"additions"`
	Deletions    int    `json:"deletions"`
	PatchPreview string `json:"patch_preview"`
}

// AICommit is the cleaned commit payload sent to AI
type AICommit struct {
	SHA     string       `json:"sha"`
	Date    string       `json:"date"`
	Message string       `json:"message"`
	Stats   Stats        `json:"stats"`
	Files   []FileChange `json:"files"`
}

// GitHubPayload is the full payload sent to AI
type GitHubPayload struct {
	Repo          string        `json:"repo"`
	Author        string        `json:"author"`
	RepoLanguages RepoLanguages `json:"repo_languages"`
	Commits       []AICommit    `json:"commits"`
}
