package stash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Performer represents a StashApp performer with their name and aliases.
type Performer struct {
	Name    string
	Aliases []string
}

// graphqlResponse is the response shape from the Stash performer query.
type graphqlResponse struct {
	Data struct {
		FindPerformers struct {
			Performers []struct {
				Name      string   `json:"name"`
				AliasList []string `json:"alias_list"`
			} `json:"performers"`
		} `json:"findPerformers"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// Client fetches performers from a StashApp GraphQL API with an in-memory cache.
type Client struct {
	baseURL    string
	apiKey     string
	cacheTTL   time.Duration
	httpClient *http.Client

	mu        sync.Mutex
	cached    []Performer
	fetchedAt time.Time
}

// NewClient creates a new Stash client. apiKey is kept in memory only and never logged.
func NewClient(baseURL, apiKey string, cacheTTLSec int) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		apiKey:   apiKey,
		cacheTTL: time.Duration(cacheTTLSec) * time.Second,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Performers returns the list of performers from Stash, using the in-memory
// cache when the TTL has not elapsed.
func (c *Client) Performers(ctx context.Context) ([]Performer, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cached != nil && time.Since(c.fetchedAt) < c.cacheTTL {
		return c.cached, nil
	}

	performers, err := c.fetchPerformers(ctx)
	if err != nil {
		return nil, err
	}

	c.cached = performers
	c.fetchedAt = time.Now()
	return c.cached, nil
}

// InvalidateCache clears the in-memory performer cache.
func (c *Client) InvalidateCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cached = nil
}

func (c *Client) fetchPerformers(ctx context.Context) ([]Performer, error) {
	const query = `{"query":"query{findPerformers(filter:{per_page:-1}){performers{name alias_list}}}"}`

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/graphql", bytes.NewBufferString(query))
	if err != nil {
		return nil, fmt.Errorf("building stash request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// API key is optional (internal Stash with no auth needs none). When set it
	// goes in the header only; never logged.
	if c.apiKey != "" {
		req.Header.Set("ApiKey", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stash request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stash returned HTTP %d", resp.StatusCode)
	}

	var gqlResp graphqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return nil, fmt.Errorf("decoding stash response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("stash GraphQL error: %s", gqlResp.Errors[0].Message)
	}

	raw := gqlResp.Data.FindPerformers.Performers
	performers := make([]Performer, 0, len(raw))
	for _, p := range raw {
		performers = append(performers, Performer{
			Name:    p.Name,
			Aliases: p.AliasList,
		})
	}
	return performers, nil
}
