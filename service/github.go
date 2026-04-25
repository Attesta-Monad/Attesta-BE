package service

import (
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
	githubAPI      = "https://api.github.com"
	maxCommits     = 20
	maxPatchLen    = 300
	maxFiles       = 50
	githubTimeout  = 30 * time.Second
)

var githubClient = &http.Client{Timeout: githubTimeout}

func githubGet(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if t := config.App.GitHubToken; t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	return githubClient.Do(req)
}

// FetchGitHubData fetches languages, commits, and commit details for the given repo/author.
func FetchGitHubData(owner, repo, author string) (*model.GitHubPayload, int, error) {
	languages, err := fetchLanguages(owner, repo)
	if err != nil {
		return nil, 0, err
	}

	summaries, err := fetchCommitList(owner, repo, author)
	if err != nil {
		return nil, 0, err
	}
	if len(summaries) == 0 {
		return nil, 0, fmt.Errorf("GITHUB_USER_NOT_FOUND")
	}

	commits, fetched, err := fetchCommitDetails(owner, repo, summaries)
	if err != nil {
		return nil, fetched, err
	}
	if len(commits) == 0 {
		return nil, fetched, fmt.Errorf("NO_VALID_COMMITS")
	}

	payload := &model.GitHubPayload{
		Repo:          owner + "/" + repo,
		Author:        author,
		RepoLanguages: languages,
		Commits:       commits,
	}
	return payload, fetched, nil
}

func fetchLanguages(owner, repo string) (model.RepoLanguages, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/languages", githubAPI, owner, repo)
	resp, err := githubGet(url)
	if err != nil {
		return nil, fmt.Errorf("github: fetch languages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("GITHUB_REPO_NOT_FOUND")
	}
	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("GITHUB_RATE_LIMIT")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github: fetch languages: status %d", resp.StatusCode)
	}

	var langs model.RepoLanguages
	if err := decodeJSON(resp.Body, &langs); err != nil {
		return nil, fmt.Errorf("github: fetch languages: %w", err)
	}
	log.Printf("[GitHub] languages fetched: %d entries", len(langs))
	return langs, nil
}

// rawCommit is used to parse the GitHub list commits response
type rawCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Date time.Time `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	Parents []model.Parent `json:"parents"`
}

func fetchCommitList(owner, repo, author string) ([]model.CommitSummary, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/commits?author=%s&per_page=100", githubAPI, owner, repo, author)
	resp, err := githubGet(url)
	if err != nil {
		return nil, fmt.Errorf("github: fetch commits: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("GITHUB_REPO_NOT_FOUND")
	}
	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("GITHUB_RATE_LIMIT")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github: fetch commits: status %d", resp.StatusCode)
	}

	var raw []rawCommit
	if err := decodeJSON(resp.Body, &raw); err != nil {
		return nil, fmt.Errorf("github: fetch commits: %w", err)
	}

	var summaries []model.CommitSummary
	for _, r := range raw {
		// skip merge commits
		if len(r.Parents) > 1 {
			continue
		}
		summaries = append(summaries, model.CommitSummary{
			SHA:     r.SHA,
			Message: r.Commit.Message,
			Date:    r.Commit.Author.Date,
			Parents: r.Parents,
		})
	}

	log.Printf("[GitHub] commits after merge filter: %d", len(summaries))
	return summaries, nil
}

// rawDetail is used to parse the GitHub commit detail response
type rawDetail struct {
	SHA   string `json:"sha"`
	Stats struct {
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
		Total     int `json:"total"`
	} `json:"stats"`
	Files []struct {
		Filename  string `json:"filename"`
		Status    string `json:"status"`
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
		Patch     string `json:"patch"`
	} `json:"files"`
}

func fetchCommitDetails(owner, repo string, summaries []model.CommitSummary) ([]model.AICommit, int, error) {
	// cap at maxCommits most recent
	if len(summaries) > maxCommits {
		summaries = summaries[:maxCommits]
	}

	var commits []model.AICommit
	for _, s := range summaries {
		url := fmt.Sprintf("%s/repos/%s/%s/commits/%s", githubAPI, owner, repo, s.SHA)
		resp, err := githubGet(url)
		if err != nil {
			return nil, len(summaries), fmt.Errorf("github: fetch commit detail %s: %w", s.SHA[:7], err)
		}

		var detail rawDetail
		decErr := decodeJSON(resp.Body, &detail)
		resp.Body.Close()
		if decErr != nil {
			return nil, len(summaries), fmt.Errorf("github: parse commit detail %s: %w", s.SHA[:7], decErr)
		}

		// skip auto-generated / massive commits
		if len(detail.Files) > maxFiles {
			log.Printf("[GitHub] skip %s: %d files", s.SHA[:7], len(detail.Files))
			continue
		}

		var files []model.FileChange
		for _, f := range detail.Files {
			patch := f.Patch
			if len(patch) > maxPatchLen {
				patch = patch[:maxPatchLen]
			}
			files = append(files, model.FileChange{
				Filename:     f.Filename,
				Status:       f.Status,
				Additions:    f.Additions,
				Deletions:    f.Deletions,
				PatchPreview: patch,
			})
		}

		commits = append(commits, model.AICommit{
			SHA:     s.SHA[:7],
			Date:    s.Date.UTC().Format(time.RFC3339),
			Message: s.Message,
			Stats: model.Stats{
				Additions: detail.Stats.Additions,
				Deletions: detail.Stats.Deletions,
				Total:     detail.Stats.Total,
			},
			Files: files,
		})
	}

	log.Printf("[GitHub] commit details fetched: %d", len(commits))
	return commits, len(summaries), nil
}

func decodeJSON(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}
