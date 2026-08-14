package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
)

// ─── Git Integrations Handlers (GitHub, GitLab, Bitbucket) ───────────────────

func getProviderOAuthCredentials(provider string) (string, string) {
	switch strings.ToLower(provider) {
	case "github":
		return os.Getenv("GITHUB_CLIENT_ID"), os.Getenv("GITHUB_CLIENT_SECRET")
	case "gitlab":
		return os.Getenv("GITLAB_CLIENT_ID"), os.Getenv("GITLAB_CLIENT_SECRET")
	case "bitbucket":
		return os.Getenv("BITBUCKET_CLIENT_ID"), os.Getenv("BITBUCKET_CLIENT_SECRET")
	default:
		return "", ""
	}
}

// Fetch GitHub Repositories
func fetchGitHubRepos(token string) ([]fiber.Map, error) {
	client := &nethttp.Client{Timeout: 10 * time.Second}
	req, err := nethttp.NewRequest("GET", "https://api.github.com/user/repos?sort=updated&per_page=100", nil)
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

// Fetch GitLab Repositories
func fetchGitLabRepos(token string) ([]fiber.Map, error) {
	client := &nethttp.Client{Timeout: 10 * time.Second}
	req, err := nethttp.NewRequest("GET", "https://gitlab.com/api/v4/projects?membership=true&order_by=updated_at&per_page=100", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "kloudsPanel-App")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != nethttp.StatusOK {
		return nil, fmt.Errorf("gitlab api returned status: %s", resp.Status)
	}

	var glRepos []struct {
		Name              string `json:"name"`
		PathWithNamespace string `json:"path_with_namespace"`
		WebURL            string `json:"web_url"`
		HTTPURLToRepo     string `json:"http_url_to_repo"`
		DefaultBranch     string `json:"default_branch"`
		Visibility        string `json:"visibility"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&glRepos); err != nil {
		return nil, err
	}

	repos := make([]fiber.Map, 0, len(glRepos))
	for _, r := range glRepos {
		branch := r.DefaultBranch
		if branch == "" {
			branch = "main"
		}
		repos = append(repos, fiber.Map{
			"name":           r.Name,
			"full_name":      r.PathWithNamespace,
			"url":            r.WebURL,
			"default_branch": branch,
			"language":       "GitLab",
			"is_private":     r.Visibility == "private",
		})
	}
	return repos, nil
}

// Fetch Bitbucket Repositories
func fetchBitbucketRepos(token, username string) ([]fiber.Map, error) {
	client := &nethttp.Client{Timeout: 10 * time.Second}
	urlStr := "https://api.bitbucket.org/2.0/repositories?role=member&pagelen=100"
	if username != "" && !strings.Contains(token, "Bearer") {
		urlStr = fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s?pagelen=100", username)
	}

	req, err := nethttp.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	if username != "" && !strings.HasPrefix(token, "ey") && len(token) < 50 {
		// App password basic auth
		req.SetBasicAuth(username, token)
	} else {
		// OAuth bearer token
		req.Header.Set("Authorization", "Bearer "+token)
	}

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
		branch := r.MainBranch.Name
		if branch == "" {
			branch = "main"
		}
		repos = append(repos, fiber.Map{
			"name":           r.Name,
			"full_name":      r.FullName,
			"url":            r.Links.HTML.Href,
			"default_branch": branch,
			"language":       r.Language,
			"is_private":     r.IsPrivate,
		})
	}
	return repos, nil
}

func (h *Handler) handleListGitIntegrations(c fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
	list, err := h.store.GitIntegrations().ListForUser(c.Context(), u.ID)
	if err != nil {
		list = []*domain.UserGitIntegration{}
	}

	connectedMap := make(map[string]*domain.UserGitIntegration)
	for _, item := range list {
		connectedMap[item.Provider] = item
	}

	result := []fiber.Map{}
	oauthEnabledMap := make(map[string]bool)

	for _, prov := range []string{"github", "gitlab", "bitbucket"} {
		clientID, _ := getProviderOAuthCredentials(prov)
		oauthEnabledMap[prov] = clientID != ""

		if item, exists := connectedMap[prov]; exists {
			result = append(result, fiber.Map{
				"provider":     prov,
				"connected":    true,
				"username":     item.Username,
				"avatar_url":   item.AvatarURL,
				"connected_at": item.ConnectedAt,
				"oauthEnabled": clientID != "",
			})
		} else {
			result = append(result, fiber.Map{
				"provider":     prov,
				"connected":    false,
				"username":     "",
				"oauthEnabled": clientID != "",
			})
		}
	}

	return c.JSON(fiber.Map{
		"integrations": result,
		"oauthEnabled": oauthEnabledMap,
	})
}

// Direct Multi-Provider OAuth Initiation (GitHub, GitLab, Bitbucket)
func (h *Handler) handleGitOAuthAuthorize(c fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
	provider := strings.ToLower(c.Params("provider"))
	clientID, _ := getProviderOAuthCredentials(provider)

	if clientID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": fmt.Sprintf("%s OAuth is not configured on this server. Set %s_CLIENT_ID and %s_CLIENT_SECRET in .env or connect using a Personal Access Token / App Password.", strings.ToUpper(provider), strings.ToUpper(provider), strings.ToUpper(provider)),
		})
	}

	rootDomain := getRootDomain()
	redirectURI := fmt.Sprintf("https://%s/api/v1/integrations/git/%s/callback", rootDomain, provider)
	returnTo := c.Query("return_to", "/workspaces")

	state := fmt.Sprintf("%s:%s", u.ID, url.QueryEscape(returnTo))

	var authURL string
	switch provider {
	case "github":
		authURL = fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=repo,read:user,user:email&state=%s",
			url.QueryEscape(clientID), url.QueryEscape(redirectURI), url.QueryEscape(state))
	case "gitlab":
		authURL = fmt.Sprintf("https://gitlab.com/oauth/authorize?client_id=%s&redirect_uri=%s&response_type=code&state=%s&scope=read_user+read_api+read_repository",
			url.QueryEscape(clientID), url.QueryEscape(redirectURI), url.QueryEscape(state))
	case "bitbucket":
		authURL = fmt.Sprintf("https://bitbucket.org/site/oauth2/authorize?client_id=%s&response_type=code&state=%s",
			url.QueryEscape(clientID), url.QueryEscape(state))
	default:
		return c.Status(400).JSON(fiber.Map{"error": "Unsupported Git provider: " + provider})
	}

	return c.Redirect().To(authURL)
}

// Multi-Provider OAuth Callback
func (h *Handler) handleGitOAuthCallback(c fiber.Ctx) error {
	provider := strings.ToLower(c.Params("provider"))
	code := c.Query("code")
	state := c.Query("state")
	if code == "" {
		return c.Redirect().To("/workspaces?error=missing_code")
	}

	var userID, returnTo string
	parts := strings.SplitN(state, ":", 2)
	if len(parts) >= 1 {
		userID = parts[0]
	}
	if len(parts) >= 2 {
		returnTo, _ = url.QueryUnescape(parts[1])
	}
	if returnTo == "" {
		returnTo = "/workspaces"
	}

	clientID, clientSecret := getProviderOAuthCredentials(provider)
	if clientID == "" || clientSecret == "" {
		return c.Redirect().To(returnTo + "?error=oauth_not_configured")
	}

	rootDomain := getRootDomain()
	redirectURI := fmt.Sprintf("https://%s/api/v1/integrations/git/%s/callback", rootDomain, provider)

	var accessToken, username string
	var avatarURL *string

	client := &nethttp.Client{Timeout: 15 * time.Second}

	switch provider {
	case "github":
		tokenReqBody, _ := json.Marshal(map[string]string{
			"client_id":     clientID,
			"client_secret": clientSecret,
			"code":          code,
		})
		req, err := nethttp.NewRequest("POST", "https://github.com/login/oauth/access_token", bytes.NewBuffer(tokenReqBody))
		if err != nil {
			return c.Redirect().To(returnTo + "?error=token_request_failed")
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != 200 {
			return c.Redirect().To(returnTo + "?error=oauth_exchange_failed")
		}
		defer resp.Body.Close()

		var tData struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tData); err != nil || tData.AccessToken == "" {
			return c.Redirect().To(returnTo + "?error=invalid_token_response")
		}
		accessToken = tData.AccessToken

		// User profile
		uReq, _ := nethttp.NewRequest("GET", "https://api.github.com/user", nil)
		uReq.Header.Set("Authorization", "Bearer "+accessToken)
		uReq.Header.Set("User-Agent", "kloudsPanel")
		uResp, err := client.Do(uReq)
		username = "GitHub User"
		if err == nil && uResp.StatusCode == 200 {
			var uData struct {
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
			}
			if json.NewDecoder(uResp.Body).Decode(&uData) == nil && uData.Login != "" {
				username = uData.Login
				if uData.AvatarURL != "" {
					avatarURL = &uData.AvatarURL
				}
			}
			uResp.Body.Close()
		}

	case "gitlab":
		vals := url.Values{}
		vals.Set("client_id", clientID)
		vals.Set("client_secret", clientSecret)
		vals.Set("code", code)
		vals.Set("grant_type", "authorization_code")
		vals.Set("redirect_uri", redirectURI)

		resp, err := client.PostForm("https://gitlab.com/oauth/token", vals)
		if err != nil || resp.StatusCode != 200 {
			return c.Redirect().To(returnTo + "?error=gitlab_token_failed")
		}
		defer resp.Body.Close()

		var tData struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tData); err != nil || tData.AccessToken == "" {
			return c.Redirect().To(returnTo + "?error=invalid_gitlab_token")
		}
		accessToken = tData.AccessToken

		// User profile
		uReq, _ := nethttp.NewRequest("GET", "https://gitlab.com/api/v4/user", nil)
		uReq.Header.Set("Authorization", "Bearer "+accessToken)
		uResp, err := client.Do(uReq)
		username = "GitLab User"
		if err == nil && uResp.StatusCode == 200 {
			var uData struct {
				Username  string `json:"username"`
				AvatarURL string `json:"avatar_url"`
			}
			if json.NewDecoder(uResp.Body).Decode(&uData) == nil && uData.Username != "" {
				username = uData.Username
				if uData.AvatarURL != "" {
					avatarURL = &uData.AvatarURL
				}
			}
			uResp.Body.Close()
		}

	case "bitbucket":
		vals := url.Values{}
		vals.Set("grant_type", "authorization_code")
		vals.Set("code", code)

		req, _ := nethttp.NewRequest("POST", "https://bitbucket.org/site/oauth2/access_token", strings.NewReader(vals.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(clientID, clientSecret)

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != 200 {
			return c.Redirect().To(returnTo + "?error=bitbucket_token_failed")
		}
		defer resp.Body.Close()

		var tData struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tData); err != nil || tData.AccessToken == "" {
			return c.Redirect().To(returnTo + "?error=invalid_bitbucket_token")
		}
		accessToken = tData.AccessToken

		// User profile
		uReq, _ := nethttp.NewRequest("GET", "https://api.bitbucket.org/2.0/user", nil)
		uReq.Header.Set("Authorization", "Bearer "+accessToken)
		uResp, err := client.Do(uReq)
		username = "Bitbucket User"
		if err == nil && uResp.StatusCode == 200 {
			var uData struct {
				Username    string `json:"username"`
				DisplayName string `json:"display_name"`
				Links       struct {
					Avatar struct {
						Href string `json:"href"`
					} `json:"avatar"`
				} `json:"links"`
			}
			if json.NewDecoder(uResp.Body).Decode(&uData) == nil {
				if uData.Username != "" {
					username = uData.Username
				} else if uData.DisplayName != "" {
					username = uData.DisplayName
				}
				if uData.Links.Avatar.Href != "" {
					avatarURL = &uData.Links.Avatar.Href
				}
			}
			uResp.Body.Close()
		}
	}

	// Persist per-user integration into SQLite
	gitItem := &domain.UserGitIntegration{
		UserID:    userID,
		Provider:  provider,
		Username:  username,
		Token:     accessToken,
		AvatarURL: avatarURL,
		Scopes:    "repo",
	}
	_ = h.store.GitIntegrations().Upsert(context.Background(), gitItem)

	sep := "?"
	if strings.Contains(returnTo, "?") {
		sep = "&"
	}
	return c.Redirect().To(returnTo + sep + provider + "_connected=true")
}

func (h *Handler) handleSaveGitIntegration(c fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
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

	provider := strings.ToLower(req.Provider)
	username := req.Username
	var avatarURL *string

	client := &nethttp.Client{Timeout: 8 * time.Second}

	if provider == "github" {
		uReq, _ := nethttp.NewRequest("GET", "https://api.github.com/user", nil)
		uReq.Header.Set("Authorization", "Bearer "+req.Token)
		uReq.Header.Set("User-Agent", "kloudsPanel")
		resp, err := client.Do(uReq)
		if err == nil && resp.StatusCode == 200 {
			var uData struct {
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
			}
			if json.NewDecoder(resp.Body).Decode(&uData) == nil && uData.Login != "" {
				username = uData.Login
				if uData.AvatarURL != "" {
					avatarURL = &uData.AvatarURL
				}
			}
			resp.Body.Close()
		} else if err != nil || resp.StatusCode != 200 {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid GitHub token: verification failed."})
		}
	} else if provider == "gitlab" {
		uReq, _ := nethttp.NewRequest("GET", "https://gitlab.com/api/v4/user", nil)
		uReq.Header.Set("Authorization", "Bearer "+req.Token)
		resp, err := client.Do(uReq)
		if err == nil && resp.StatusCode == 200 {
			var uData struct {
				Username  string `json:"username"`
				AvatarURL string `json:"avatar_url"`
			}
			if json.NewDecoder(resp.Body).Decode(&uData) == nil && uData.Username != "" {
				username = uData.Username
				if uData.AvatarURL != "" {
					avatarURL = &uData.AvatarURL
				}
			}
			resp.Body.Close()
		}
	} else if provider == "bitbucket" {
		uReq, _ := nethttp.NewRequest("GET", "https://api.bitbucket.org/2.0/user", nil)
		if req.Username != "" {
			uReq.SetBasicAuth(req.Username, req.Token)
		} else {
			uReq.Header.Set("Authorization", "Bearer "+req.Token)
		}
		resp, err := client.Do(uReq)
		if err == nil && resp.StatusCode == 200 {
			var uData struct {
				Username string `json:"username"`
			}
			if json.NewDecoder(resp.Body).Decode(&uData) == nil && uData.Username != "" {
				username = uData.Username
			}
			resp.Body.Close()
		}
	}

	gitItem := &domain.UserGitIntegration{
		UserID:    u.ID,
		Provider:  provider,
		Username:  username,
		Token:     req.Token,
		AvatarURL: avatarURL,
		Scopes:    "repo",
	}
	if err := h.store.GitIntegrations().Upsert(c.Context(), gitItem); err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"provider":     provider,
		"connected":    true,
		"username":     username,
		"avatar_url":   avatarURL,
		"connected_at": time.Now().UTC(),
	})
}

func (h *Handler) handleDeleteGitIntegration(c fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
	p := strings.ToLower(c.Params("provider"))
	_ = h.store.GitIntegrations().Delete(c.Context(), u.ID, p)
	return c.SendStatus(204)
}

func (h *Handler) handleListGitRepos(c fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
	provider := strings.ToLower(c.Params("provider"))

	integration, err := h.store.GitIntegrations().Get(c.Context(), u.ID, provider)
	if err != nil || integration == nil || integration.Token == "" {
		return c.JSON(fiber.Map{"provider": provider, "connected": false, "repos": []any{}})
	}

	if provider == "github" {
		repos, err := fetchGitHubRepos(integration.Token)
		if err != nil {
			return c.JSON(fiber.Map{"provider": provider, "connected": true, "repos": []any{}, "error": err.Error()})
		}
		return c.JSON(fiber.Map{"provider": provider, "connected": true, "username": integration.Username, "avatar_url": integration.AvatarURL, "repos": repos})
	} else if provider == "gitlab" {
		repos, err := fetchGitLabRepos(integration.Token)
		if err != nil {
			return c.JSON(fiber.Map{"provider": provider, "connected": true, "repos": []any{}, "error": err.Error()})
		}
		return c.JSON(fiber.Map{"provider": provider, "connected": true, "username": integration.Username, "avatar_url": integration.AvatarURL, "repos": repos})
	} else if provider == "bitbucket" {
		repos, err := fetchBitbucketRepos(integration.Token, integration.Username)
		if err != nil {
			return c.JSON(fiber.Map{"provider": provider, "connected": true, "repos": []any{}, "error": err.Error()})
		}
		return c.JSON(fiber.Map{"provider": provider, "connected": true, "username": integration.Username, "avatar_url": integration.AvatarURL, "repos": repos})
	}

	return c.JSON(fiber.Map{"provider": provider, "connected": true, "repos": []any{}})
}
