package http

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
	"github.com/yourorg/klouds/api/internal/domain/ids"
)

// normalizeGitRepoURL strips protocols, .git, and trailing slashes for standard comparison.
func normalizeGitRepoURL(url string) string {
	clean := strings.TrimSpace(url)
	clean = strings.TrimSuffix(clean, "/")
	clean = strings.TrimSuffix(clean, ".git")
	clean = strings.ToLower(clean)
	if strings.HasPrefix(clean, "https://") {
		clean = strings.TrimPrefix(clean, "https://")
	} else if strings.HasPrefix(clean, "http://") {
		clean = strings.TrimPrefix(clean, "http://")
	} else if strings.HasPrefix(clean, "git@") {
		clean = strings.TrimPrefix(clean, "git@")
		clean = strings.Replace(clean, ":", "/", 1)
	}
	return clean
}

// Direct Service Webhook: POST /api/v1/webhooks/deploy/:serviceId
func (h *Handler) handleServiceDeployWebhook(c fiber.Ctx) error {
	serviceID := c.Params("serviceId")
	if serviceID == "" {
		serviceID = c.Params("id")
	}

	s, err := h.store.Services().GetByID(c.Context(), serviceID)
	if err != nil || s == nil {
		sList, _ := h.store.Services().ListAll(c.Context())
		for _, candidate := range sList {
			if candidate.Slug == serviceID {
				s = candidate
				break
			}
		}
	}
	if s == nil {
		return c.Status(404).JSON(fiber.Map{"error": fmt.Sprintf("Service '%s' not found", serviceID)})
	}

	var resMap map[string]any
	if s.ResourceJSON != "" {
		_ = json.Unmarshal([]byte(s.ResourceJSON), &resMap)
	}
	if resMap == nil {
		resMap = make(map[string]any)
	}

	serviceBranch, _ := resMap["gitBranch"].(string)
	if serviceBranch == "" {
		serviceBranch = "main"
	}

	// Parse optional git push event payload
	var commitSHA, commitMsg, commitAuthor, pushedBranch string
	rawBody := c.Body()
	if len(rawBody) > 0 {
		var payload struct {
			Ref        string `json:"ref"`
			Branch     string `json:"branch"`
			After      string `json:"after"`
			HeadCommit *struct {
				ID      string `json:"id"`
				Message string `json:"message"`
				Author  struct {
					Name  string `json:"name"`
					Email string `json:"email"`
				} `json:"author"`
			} `json:"head_commit"`
			Commits []struct {
				ID      string `json:"id"`
				Message string `json:"message"`
				Author  struct {
					Name string `json:"name"`
				} `json:"author"`
			} `json:"commits"`
			Push struct {
				Changes []struct {
					New struct {
						Name   string `json:"name"`
						Target struct {
							Hash string `json:"hash"`
						} `json:"target"`
					} `json:"new"`
				} `json:"changes"`
			} `json:"push"`
		}
		if json.Unmarshal(rawBody, &payload) == nil {
			if strings.HasPrefix(payload.Ref, "refs/heads/") {
				pushedBranch = strings.TrimPrefix(payload.Ref, "refs/heads/")
			} else if payload.Branch != "" {
				pushedBranch = payload.Branch
			} else if len(payload.Push.Changes) > 0 && payload.Push.Changes[0].New.Name != "" {
				pushedBranch = payload.Push.Changes[0].New.Name
			}

			if payload.HeadCommit != nil {
				commitSHA = payload.HeadCommit.ID
				commitMsg = payload.HeadCommit.Message
				commitAuthor = payload.HeadCommit.Author.Name
			} else if len(payload.Commits) > 0 {
				commitSHA = payload.Commits[len(payload.Commits)-1].ID
				commitMsg = payload.Commits[len(payload.Commits)-1].Message
				commitAuthor = payload.Commits[len(payload.Commits)-1].Author.Name
			} else if len(payload.Push.Changes) > 0 && payload.Push.Changes[0].New.Target.Hash != "" {
				commitSHA = payload.Push.Changes[0].New.Target.Hash
			}
			if payload.After != "" && commitSHA == "" {
				commitSHA = payload.After
			}
		}
	}

	// Verify branch if specified in payload
	if pushedBranch != "" && !strings.EqualFold(pushedBranch, serviceBranch) {
		return c.JSON(fiber.Map{
			"status":  "ignored",
			"message": fmt.Sprintf("Push event for branch '%s' ignored (service is tracking '%s')", pushedBranch, serviceBranch),
		})
	}

	if len(commitSHA) > 7 {
		commitSHA = commitSHA[:7]
	}
	if commitAuthor == "" {
		commitAuthor = "git-webhook"
	}

	seq, _ := h.store.Deployments().GetNextSequence(c.Context(), s.ID)
	rootDomain := getRootDomain()

	snapMap := map[string]any{
		"commit_sha":    commitSHA,
		"commit_msg":    commitMsg,
		"commit_author": commitAuthor,
		"pushed_branch": pushedBranch,
		"deployed_via":  "webhook",
	}
	snapBytes, _ := json.Marshal(snapMap)

	dep := &domain.Deployment{
		ID:             ids.NewV7(),
		ServiceID:      s.ID,
		Sequence:       seq,
		Trigger:        domain.TriggerAuto,
		Status:         domain.DeploymentQueued,
		BuildDriver:    "docker",
		ConfigSnapshot: string(snapBytes),
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := h.store.Deployments().Create(c.Context(), dep); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create deployment record: " + err.Error()})
	}

	go h.executeDeployment(s, dep, rootDomain)

	return c.JSON(fiber.Map{
		"status":        "queued",
		"deployment_id": dep.ID,
		"sequence":      dep.Sequence,
		"service_id":    s.ID,
		"service_slug":  s.Slug,
		"message":       "Auto-deployment successfully initiated.",
	})
}

// Generic Git Provider Webhook: POST /api/v1/webhooks/git & /api/v1/webhooks/git/:provider
func (h *Handler) handleGenericGitWebhook(c fiber.Ctx) error {
	rawBody := c.Body()
	if len(rawBody) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Empty webhook payload"})
	}

	var payload struct {
		Ref        string `json:"ref"`
		Repository struct {
			HTMLURL  string `json:"html_url"`
			CloneURL string `json:"clone_url"`
			SSHURL   string `json:"ssh_url"`
			FullName string `json:"full_name"`
			Links    struct {
				HTML struct {
					Href string `json:"href"`
				} `json:"html"`
			} `json:"links"`
		} `json:"repository"`
		Project struct {
			WebURL     string `json:"web_url"`
			GitHTTPURL string `json:"git_http_url"`
			PathWithNS string `json:"path_with_namespace"`
		} `json:"project"`
		After      string `json:"after"`
		HeadCommit *struct {
			ID      string `json:"id"`
			Message string `json:"message"`
			Author  struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"author"`
		} `json:"head_commit"`
		Commits []struct {
			ID      string `json:"id"`
			Message string `json:"message"`
			Author  struct {
				Name string `json:"name"`
			} `json:"author"`
		} `json:"commits"`
		Push struct {
			Changes []struct {
				New struct {
					Name   string `json:"name"`
					Target struct {
						Hash string `json:"hash"`
					} `json:"target"`
				} `json:"new"`
			} `json:"changes"`
		} `json:"push"`
	}

	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid webhook JSON: " + err.Error()})
	}

	var pushedBranch string
	if strings.HasPrefix(payload.Ref, "refs/heads/") {
		pushedBranch = strings.TrimPrefix(payload.Ref, "refs/heads/")
	} else if len(payload.Push.Changes) > 0 && payload.Push.Changes[0].New.Name != "" {
		pushedBranch = payload.Push.Changes[0].New.Name
	}
	if pushedBranch == "" {
		pushedBranch = "main"
	}

	// Candidate URLs for matching
	var candidateURLs []string
	if payload.Repository.CloneURL != "" {
		candidateURLs = append(candidateURLs, normalizeGitRepoURL(payload.Repository.CloneURL))
	}
	if payload.Repository.HTMLURL != "" {
		candidateURLs = append(candidateURLs, normalizeGitRepoURL(payload.Repository.HTMLURL))
	}
	if payload.Repository.SSHURL != "" {
		candidateURLs = append(candidateURLs, normalizeGitRepoURL(payload.Repository.SSHURL))
	}
	if payload.Repository.FullName != "" {
		candidateURLs = append(candidateURLs, normalizeGitRepoURL("github.com/"+payload.Repository.FullName))
		candidateURLs = append(candidateURLs, normalizeGitRepoURL(payload.Repository.FullName))
	}
	if payload.Repository.Links.HTML.Href != "" {
		candidateURLs = append(candidateURLs, normalizeGitRepoURL(payload.Repository.Links.HTML.Href))
	}
	if payload.Project.WebURL != "" {
		candidateURLs = append(candidateURLs, normalizeGitRepoURL(payload.Project.WebURL))
	}
	if payload.Project.GitHTTPURL != "" {
		candidateURLs = append(candidateURLs, normalizeGitRepoURL(payload.Project.GitHTTPURL))
	}
	if payload.Project.PathWithNS != "" {
		candidateURLs = append(candidateURLs, normalizeGitRepoURL("gitlab.com/"+payload.Project.PathWithNS))
	}

	var commitSHA, commitMsg, commitAuthor string
	if payload.HeadCommit != nil {
		commitSHA = payload.HeadCommit.ID
		commitMsg = payload.HeadCommit.Message
		commitAuthor = payload.HeadCommit.Author.Name
	} else if len(payload.Commits) > 0 {
		commitSHA = payload.Commits[len(payload.Commits)-1].ID
		commitMsg = payload.Commits[len(payload.Commits)-1].Message
		commitAuthor = payload.Commits[len(payload.Commits)-1].Author.Name
	} else if len(payload.Push.Changes) > 0 && payload.Push.Changes[0].New.Target.Hash != "" {
		commitSHA = payload.Push.Changes[0].New.Target.Hash
	}
	if payload.After != "" && commitSHA == "" {
		commitSHA = payload.After
	}
	if len(commitSHA) > 7 {
		commitSHA = commitSHA[:7]
	}
	if commitAuthor == "" {
		commitAuthor = "git-webhook"
	}

	// Find matching services
	allServices, err := h.store.Services().ListAll(c.Context())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list services: " + err.Error()})
	}

	var triggered []fiber.Map
	rootDomain := getRootDomain()

	for _, s := range allServices {
		// Only auto-deploy if enabled
		if !s.AutoDeploy {
			continue
		}

		var resMap map[string]any
		if s.ResourceJSON != "" {
			_ = json.Unmarshal([]byte(s.ResourceJSON), &resMap)
		}
		if resMap == nil {
			continue
		}

		svcRepoURL, _ := resMap["gitRepoUrl"].(string)
		if svcRepoURL == "" {
			continue
		}
		normalizedSvcRepo := normalizeGitRepoURL(svcRepoURL)

		matched := false
		for _, cand := range candidateURLs {
			if cand == normalizedSvcRepo || strings.HasSuffix(normalizedSvcRepo, cand) || strings.HasSuffix(cand, normalizedSvcRepo) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		svcBranch, _ := resMap["gitBranch"].(string)
		if svcBranch == "" {
			svcBranch = "main"
		}
		if !strings.EqualFold(svcBranch, pushedBranch) {
			continue
		}

		seq, _ := h.store.Deployments().GetNextSequence(c.Context(), s.ID)

		snapMap := map[string]any{
			"commit_sha":    commitSHA,
			"commit_msg":    commitMsg,
			"commit_author": commitAuthor,
			"pushed_branch": pushedBranch,
			"deployed_via":  "git_push_webhook",
		}
		snapBytes, _ := json.Marshal(snapMap)

		dep := &domain.Deployment{
			ID:             ids.NewV7(),
			ServiceID:      s.ID,
			Sequence:       seq,
			Trigger:        domain.TriggerAuto,
			Status:         domain.DeploymentQueued,
			BuildDriver:    "docker",
			ConfigSnapshot: string(snapBytes),
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}

		if err := h.store.Deployments().Create(c.Context(), dep); err != nil {
			continue
		}

		go h.executeDeployment(s, dep, rootDomain)

		triggered = append(triggered, fiber.Map{
			"service_id":    s.ID,
			"service_slug":  s.Slug,
			"deployment_id": dep.ID,
			"sequence":      dep.Sequence,
		})
	}

	return c.JSON(fiber.Map{
		"status":    "processed",
		"triggered": triggered,
		"count":     len(triggered),
	})
}
