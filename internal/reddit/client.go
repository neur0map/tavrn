package reddit

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultCacheTTL = 1 * time.Hour
	httpTimeout     = 10 * time.Second
	userAgent       = "tavrn:v0.5 (terminal tavern, github.com/neur0map/tavrn)"
	maxImageBytes   = 2 * 1024 * 1024 // 2MB
	maxPosts        = 100
)

// Client is a Reddit API client with caching.
// Shared across all SSH sessions — post and thumbnail caches are server-wide.
type Client struct {
	http     *http.Client
	cacheTTL time.Duration

	// OAuth credentials (optional — falls back to public JSON API)
	clientID     string
	clientSecret string
	tokenMu      sync.Mutex
	accessToken  string
	tokenExpiry  time.Time

	mu        sync.RWMutex
	postCache []Post
	cacheTime time.Time

	// Server-shared rendered thumbnail cache: post ID → ANSI string.
	// First user to view a post triggers the render; everyone else gets cache.
	thumbMu    sync.RWMutex
	thumbCache map[string]string // "" = loading, non-empty = rendered
}

// NewClient creates a Reddit client with 24h cache TTL and 10s HTTP timeout.
// If clientID and clientSecret are provided, uses OAuth (oauth.reddit.com)
// which is more reliable from cloud servers. Otherwise falls back to
// the public JSON API (www.reddit.com) which may get 403 on some IPs.
func NewClient(clientID ...string) *Client {
	c := &Client{
		http: &http.Client{
			Timeout: httpTimeout,
		},
		cacheTTL:   defaultCacheTTL,
		thumbCache: make(map[string]string),
	}
	if len(clientID) >= 2 && clientID[0] != "" && clientID[1] != "" {
		c.clientID = clientID[0]
		c.clientSecret = clientID[1]
		log.Printf("reddit: using OAuth (client ID: %s...)", c.clientID[:8])
	}
	return c
}

// getToken returns a valid OAuth access token, refreshing if needed.
func (c *Client) getToken() (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	data := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.clientID, c.clientSecret)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("reddit oauth: %w", err)
	}
	defer resp.Body.Close()

	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", fmt.Errorf("reddit oauth decode: %w", err)
	}
	if tok.Error != "" {
		return "", fmt.Errorf("reddit oauth: %s", tok.Error)
	}

	c.accessToken = tok.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tok.ExpiresIn-60) * time.Second)
	return c.accessToken, nil
}

// useOAuth returns true if OAuth credentials are configured.
func (c *Client) useOAuth() bool {
	return c.clientID != "" && c.clientSecret != ""
}

// apiBase returns the base URL — oauth.reddit.com with OAuth, www.reddit.com without.
func (c *Client) apiBase() string {
	if c.useOAuth() {
		return "https://oauth.reddit.com"
	}
	return "https://www.reddit.com"
}

// setAuth sets the appropriate auth headers on a request.
func (c *Client) setAuth(req *http.Request) error {
	req.Header.Set("User-Agent", userAgent)
	if c.useOAuth() {
		token, err := c.getToken()
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}

// GetThumb returns the cached rendered thumbnail for a post.
// Returns (rendered, true) if cached and rendered, ("", true) if loading, ("", false) if not started.
func (c *Client) GetThumb(postID string) (string, bool) {
	c.thumbMu.RLock()
	defer c.thumbMu.RUnlock()
	v, ok := c.thumbCache[postID]
	return v, ok
}

// SetThumb stores a rendered thumbnail in the server-wide cache.
func (c *Client) SetThumb(postID, rendered string) {
	c.thumbMu.Lock()
	defer c.thumbMu.Unlock()
	c.thumbCache[postID] = rendered
}

// MarkThumbLoading marks a thumbnail as loading to prevent duplicate fetches.
// Returns true if this call claimed the load (wasn't already started).
func (c *Client) MarkThumbLoading(postID string) bool {
	c.thumbMu.Lock()
	defer c.thumbMu.Unlock()
	if _, ok := c.thumbCache[postID]; ok {
		return false // already loading or done
	}
	c.thumbCache[postID] = "" // mark loading
	return true
}

// FetchSubreddit fetches hot posts from a single subreddit.
func (c *Client) FetchSubreddit(subreddit string, limit int) ([]Post, error) {
	u := fmt.Sprintf("%s/r/%s/hot.json?limit=%d&raw_json=1", c.apiBase(), subreddit, limit)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("reddit: build request: %w", err)
	}
	if err := c.setAuth(req); err != nil {
		return nil, fmt.Errorf("reddit: auth: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reddit: fetch r/%s: %w", subreddit, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reddit: r/%s returned %d", subreddit, resp.StatusCode)
	}

	var listing struct {
		Data struct {
			Children []struct {
				Data jsonPost `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listing); err != nil {
		return nil, fmt.Errorf("reddit: decode r/%s: %w", subreddit, err)
	}

	posts := make([]Post, 0, len(listing.Data.Children))
	for _, child := range listing.Data.Children {
		posts = append(posts, child.Data.toPost())
	}
	return posts, nil
}

// FetchMerged fetches from multiple subreddits sequentially with a delay
// between each to avoid rate limits. New posts are merged into the existing
// cache (deduped by ID), sorted by score, and capped at 100.
func (c *Client) FetchMerged(subreddits []string, perSub int) ([]Post, error) {
	// Start with existing cached posts
	c.mu.RLock()
	seen := make(map[string]bool)
	var merged []Post
	for _, p := range c.postCache {
		seen[p.ID] = true
		merged = append(merged, p)
	}
	c.mu.RUnlock()

	var firstErr error
	for i, sub := range subreddits {
		if i > 0 {
			time.Sleep(2 * time.Second) // stagger requests
		}
		posts, err := c.FetchSubreddit(sub, perSub)
		if err != nil {
			log.Printf("reddit: r/%s failed: %v", sub, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, p := range posts {
			if !seen[p.ID] {
				seen[p.ID] = true
				merged = append(merged, p)
			}
		}
	}

	// If we got no posts at all (including from cache), return the first error.
	if len(merged) == 0 && firstErr != nil {
		return nil, firstErr
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	if len(merged) > maxPosts {
		merged = merged[:maxPosts]
	}

	c.mu.Lock()
	c.postCache = merged
	c.cacheTime = time.Now()
	c.mu.Unlock()

	return merged, nil
}

// Posts returns cached posts.
func (c *Client) Posts() ([]Post, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.postCache == nil {
		return nil, fmt.Errorf("reddit: no cached posts")
	}
	out := make([]Post, len(c.postCache))
	copy(out, c.postCache)
	return out, nil
}

// NeedsRefresh returns true if the cache is older than the TTL.
func (c *Client) NeedsRefresh() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.cacheTime.IsZero() {
		return true
	}
	return time.Since(c.cacheTime) > c.cacheTTL
}

// CacheAge returns how old the cache is.
func (c *Client) CacheAge() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.cacheTime.IsZero() {
		return 0
	}
	return time.Since(c.cacheTime)
}

// FetchComments fetches comments for a post, parsing nested replies up to depth 2.
func (c *Client) FetchComments(subreddit, postID string, limit int) ([]Comment, error) {
	u := fmt.Sprintf(
		"%s/r/%s/comments/%s.json?raw_json=1&limit=%d&depth=3",
		c.apiBase(), subreddit, postID, limit,
	)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("reddit: build comment request: %w", err)
	}
	if err := c.setAuth(req); err != nil {
		return nil, fmt.Errorf("reddit: auth: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reddit: fetch comments: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reddit: comments returned %d", resp.StatusCode)
	}

	// Reddit returns an array of two listings: [post, comments]
	var listings []struct {
		Data struct {
			Children []struct {
				Data jsonComment `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listings); err != nil {
		return nil, fmt.Errorf("reddit: decode comments: %w", err)
	}

	if len(listings) < 2 {
		return nil, nil
	}

	comments := make([]Comment, 0, len(listings[1].Data.Children))
	for _, child := range listings[1].Data.Children {
		if child.Data.Author == "" {
			continue // skip "more" stubs
		}
		comments = append(comments, child.Data.toComment(0))
	}
	return comments, nil
}

// FetchImage downloads and decodes a JPEG or PNG image, limited to 2MB.
func (c *Client) FetchImage(imageURL string) (image.Image, error) {
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("reddit: build image request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reddit: fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reddit: image returned %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxImageBytes+1)
	img, _, err := image.Decode(limited)
	if err != nil {
		return nil, fmt.Errorf("reddit: decode image: %w", err)
	}
	return img, nil
}

// --- JSON mapping types ---

type jsonPost struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	Subreddit   string  `json:"subreddit"`
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
	CreatedUTC  float64 `json:"created_utc"`
	URL         string  `json:"url"`
	Permalink   string  `json:"permalink"`
	Selftext    string  `json:"selftext"`
	IsSelf      bool    `json:"is_self"`
	Thumbnail   string  `json:"thumbnail"`
	PostHint    string  `json:"post_hint"`
	Domain      string  `json:"domain"`
	IsVideo     bool    `json:"is_video"`
	Preview     *struct {
		Images []struct {
			Source struct {
				URL    string `json:"url"`
				Width  int    `json:"width"`
				Height int    `json:"height"`
			} `json:"source"`
			Resolutions []struct {
				URL    string `json:"url"`
				Width  int    `json:"width"`
				Height int    `json:"height"`
			} `json:"resolutions"`
		} `json:"images"`
	} `json:"preview"`
}

func (jp *jsonPost) toPost() Post {
	p := Post{
		ID:          jp.ID,
		Title:       jp.Title,
		Author:      jp.Author,
		Subreddit:   jp.Subreddit,
		Score:       jp.Score,
		NumComments: jp.NumComments,
		CreatedUTC:  time.Unix(int64(jp.CreatedUTC), 0),
		URL:         jp.URL,
		Permalink:   jp.Permalink,
		Selftext:    jp.Selftext,
		IsSelf:      jp.IsSelf,
		Thumbnail:   jp.Thumbnail,
		PostHint:    jp.PostHint,
		Domain:      jp.Domain,
		IsVideo:     jp.IsVideo,
	}

	// Extract best preview image — closest to 1080px wide for sharp renders.
	if jp.Preview != nil && len(jp.Preview.Images) > 0 {
		img := jp.Preview.Images[0]

		bestURL := img.Source.URL
		bestW := img.Source.Width
		bestH := img.Source.Height
		bestDiff := math.Abs(float64(bestW) - 1080)

		for _, res := range img.Resolutions {
			diff := math.Abs(float64(res.Width) - 1080)
			if diff < bestDiff {
				bestURL = res.URL
				bestW = res.Width
				bestH = res.Height
				bestDiff = diff
			}
		}

		p.PreviewURL = bestURL
		p.PreviewW = bestW
		p.PreviewH = bestH
		p.HasImage = true
	}

	// Direct i.redd.it uploads are images even without preview data.
	if !p.HasImage && strings.Contains(jp.Domain, "i.redd.it") {
		p.HasImage = true
		if p.PreviewURL == "" {
			p.PreviewURL = jp.URL
		}
	}

	return p
}

type jsonComment struct {
	Author     string          `json:"author"`
	Body       string          `json:"body"`
	Score      int             `json:"score"`
	CreatedUTC float64         `json:"created_utc"`
	Depth      int             `json:"depth"`
	Replies    json.RawMessage `json:"replies"`
}

func (jc *jsonComment) toComment(depth int) Comment {
	cm := Comment{
		Author:  jc.Author,
		Body:    jc.Body,
		Score:   jc.Score,
		Created: time.Unix(int64(jc.CreatedUTC), 0),
		Depth:   depth,
	}

	// Parse nested replies — replies can be a struct or empty string.
	if depth < 2 && len(jc.Replies) > 0 {
		var replies struct {
			Data struct {
				Children []struct {
					Data jsonComment `json:"data"`
				} `json:"children"`
			} `json:"data"`
		}
		if json.Unmarshal(jc.Replies, &replies) == nil {
			for _, child := range replies.Data.Children {
				if child.Data.Author == "" {
					continue
				}
				cm.Children = append(cm.Children, child.Data.toComment(depth+1))
			}
		}
	}

	return cm
}
