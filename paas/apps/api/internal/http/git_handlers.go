package http

import (
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"time"

	"github.com/gofiber/fiber/v3"
)

// ─── Git Integrations Handlers ────────────────────────────────────────────────

type GitIntegration struct {
	Provider    string    `json:"provider"`
	Connected   bool      `json:"connected"`
	Username    string    `json:"username"`
	Token       string    `json:"-"`
	ConnectedAt time.Time `json:"connected_at"`
}

var gitIntegrationsStore = map[string]GitIntegration{
	"github": {
		Provider:  "github",
		Connected: false,
		Username:  "",
	},
	"bitbucket": {
		Provider:  "bitbucket",
		Connected: false,
		Username:  "",
	},
	"gitlab": {
		Provider:  "gitlab",
		Connected: false,
		Username:  "",
	},
}

func fetchGitHubRepos(token string) ([]fiber.Map, error) {
	client := &nethttp.Client{Timeout: 10 * time.Second}
	req, err := nethttp.NewRequest("GET", "https://api.github.com/user/repos?sort=updated&per_page=50", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "kloudsPanel-App")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != nethttp.StatusOK {
		return nil, fmt.Errorf("github api returned status: %s", resp.Status)
	}

	var ghRepos []struct {
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		HTMLURL       string `json:"html_url"`
		DefaultBranch string `json:"default_branch"`
		Language      string `json:"language"`
		Private       bool   `json:"private"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ghRepos); err != nil {
		return nil, err
	}

	repos := make([]fiber.Map, 0, len(ghRepos))
	for _, r := range ghRepos {
		repos = append(repos, fiber.Map{
			"name":           r.Name,
			"full_name":      r.FullName,
			"url":            r.HTMLURL,
			"default_branch": r.DefaultBranch,
			"language":       r.Language,
			"is_private":     r.Private,
		})
	}
	return repos, nil
}

func fetchBitbucketRepos(username, password string) ([]fiber.Map, error) {
	client := &nethttp.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s?pagelen=50", username)
	req, err := nethttp.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(username, password)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != nethttp.StatusOK {
		return nil, fmt.Errorf("bitbucket api returned status: %s", resp.Status)
	}

	var bbResp struct {
		Values []struct {
			Name     string `json:"name"`
			FullName string `json:"full_name"`
			Links    struct {
				HTML struct {
					Href string `json:"href"`
				} `json:"html"`
			} `json:"links"`
			MainBranch struct {
				Name string `json:"name"`
			} `json:"mainbranch"`
			Language  string `json:"language"`
			IsPrivate bool   `json:"is_private"`
		} `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bbResp); err != nil {
		return nil, err
	}

	repos := make([]fiber.Map, 0, len(bbResp.Values))
	for _, r := range bbResp.Values {
		repos = append(repos, fiber.Map{
			"name":           r.Name,
			"full_name":      r.FullName,
			"url":            r.Links.HTML.Href,
			"default_branch": r.MainBranch.Name,
			"language":       r.Language,
			"is_private":     r.IsPrivate,
		})
	}
	return repos, nil
}

func (h *Handler) handleListGitIntegrations(c fiber.Ctx) error {
	list := make([]GitIntegration, 0, len(gitIntegrationsStore))
	for _, v := range gitIntegrationsStore {
		list = append(list, v)
	}
	return c.JSON(fiber.Map{"integrations": list})
}

func (h *Handler) handleSaveGitIntegration(c fiber.Ctx) error {
	var req struct {
		Provider string `json:"provider"`
		Token    string `json:"token"`
		Username string `json:"username"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	if req.Token == "" {
		return c.Status(400).JSON(fiber.Map{"error": "access token is required"})
	}

	username := req.Username
	if req.Provider == "github" {
		client := &nethttp.Client{Timeout: 8 * time.Second}
		uReq, _ := nethttp.NewRequest("GET", "https://api.github.com/user", nil)
		uReq.Header.Set("Authorization", "Bearer "+req.Token)
		uReq.Header.Set("User-Agent", "kloudsPanel")
		resp, err := client.Do(uReq)
		if err == nil && resp.StatusCode == 200 {
			var uData struct {
				Login string `json:"login"`
			}
			if json.NewDecoder(resp.Body).Decode(&uData) == nil && uData.Login != "" {
				username = uData.Login
			}
			resp.Body.Close()
		}
	}

	gitIntegrationsStore[req.Provider] = GitIntegration{
		Provider:    req.Provider,
		Connected:   true,
		Username:    username,
		Token:       req.Token,
		ConnectedAt: time.Now().UTC(),
	}
	return c.JSON(gitIntegrationsStore[req.Provider])
}

func (h *Handler) handleDeleteGitIntegration(c fiber.Ctx) error {
	p := c.Params("provider")
	gitIntegrationsStore[p] = GitIntegration{
		Provider:  p,
		Connected: false,
		Username:  "",
		Token:     "",
	}
	return c.SendStatus(204)
}

func (h *Handler) handleListGitRepos(c fiber.Ctx) error {
	provider := c.Params("provider")
	integration, ok := gitIntegrationsStore[provider]
	if !ok || !integration.Connected || integration.Token == "" {
		return c.JSON(fiber.Map{"provider": provider, "repos": []any{}})
	}

	if provider == "github" {
		repos, err := fetchGitHubRepos(integration.Token)
		if err != nil {
			return c.JSON(fiber.Map{"provider": provider, "repos": []any{}, "error": err.Error()})
		}
		return c.JSON(fiber.Map{"provider": provider, "repos": repos})
	} else if provider == "bitbucket" {
		repos, err := fetchBitbucketRepos(integration.Username, integration.Token)
		if err != nil {
			return c.JSON(fiber.Map{"provider": provider, "repos": []any{}, "error": err.Error()})
		}
		return c.JSON(fiber.Map{"provider": provider, "repos": repos})
	}

	return c.JSON(fiber.Map{"provider": provider, "repos": []any{}})
}
