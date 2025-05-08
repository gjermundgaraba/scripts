package github

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gjermundgaraba/changelog-checker/pkg/db"
)

// Client is a GitHub API client with caching
type Client struct {
	httpClient   *http.Client
	token        string
	db           *db.DB
	rateLimited  bool
	resetTime    time.Time
	defaultOwner string
	defaultRepo  string
}

// NewClient creates a new GitHub API client with caching
func NewClient(token, defaultOwner, defaultRepo string, db *db.DB) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		token:        token,
		db:           db,
		defaultOwner: defaultOwner,
		defaultRepo:  defaultRepo,
	}
}

// TestToken tests if the provided GitHub token is valid
func (c *Client) TestToken() (bool, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", c.defaultOwner, c.defaultRepo)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}
	
	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	
	return resp.StatusCode == http.StatusOK, nil
}

// PRResponse represents the GitHub API response for a PR
type PRResponse struct {
	Title string `json:"title"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// GetPRInfo gets PR info with caching
func (c *Client) GetPRInfo(owner, repo string, prNumber int) (string, error) {
	// If we're rate limited and the reset time hasn't passed, return error
	if c.rateLimited && time.Now().Before(c.resetTime) {
		return "", fmt.Errorf("rate limited until %s", c.resetTime.Format(time.RFC3339))
	}

	// Check cache first
	title, found, err := c.db.GetPRInfo(owner, repo, prNumber)
	if err != nil {
		log.Printf("Error checking cache: %v", err)
	} else if found {
		return title, nil
	}

	// Not in cache or error, fetch from GitHub
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, prNumber)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	
	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	// Check for rate limiting
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		// Parse the rate limit reset time
		resetHeader := resp.Header.Get("X-RateLimit-Reset")
		if resetHeader != "" {
			resetTime, err := strconv.ParseInt(resetHeader, 10, 64)
			if err == nil {
				c.resetTime = time.Unix(resetTime, 0)
				c.rateLimited = true
				return "", fmt.Errorf("rate limited until %s", c.resetTime.Format(time.RFC3339))
			}
		}
		return "", fmt.Errorf("rate limited by GitHub API")
	}
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	
	var prResponse PRResponse
	if err := json.Unmarshal(body, &prResponse); err != nil {
		return "", err
	}
	
	// Cache the result
	if err := c.db.StorePRInfo(owner, repo, prNumber, prResponse.Title); err != nil {
		log.Printf("Error caching PR info: %v", err)
	}
	
	return prResponse.Title, nil
}