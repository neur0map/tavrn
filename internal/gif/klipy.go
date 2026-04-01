package gif

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	klipyBaseURL   = "https://api.klipy.com/api/v1"
	maxGIFSize     = 5 * 1024 * 1024 // 5MB max download
	searchTimeout  = 10 * time.Second
	fetchTimeout   = 15 * time.Second
	defaultPerPage = 8
)

// KlipyClient searches and fetches GIFs from the Klipy API.
type KlipyClient struct {
	apiKey string
	http   *http.Client
}

// KlipyResult represents a single GIF search result.
type KlipyResult struct {
	Slug   string
	Title  string
	URL    string // URL to the sm gif
	Width  int
	Height int
}

// NewKlipyClient creates a new Klipy API client.
func NewKlipyClient(apiKey string) *KlipyClient {
	return &KlipyClient{
		apiKey: apiKey,
		http:   &http.Client{Timeout: searchTimeout},
	}
}

// Search queries the Klipy API for GIFs matching the query.
func (c *KlipyClient) Search(query string) ([]KlipyResult, error) {
	endpoint := fmt.Sprintf("%s/%s/gifs/search", klipyBaseURL, c.apiKey)

	params := url.Values{}
	params.Set("q", query)
	params.Set("per_page", fmt.Sprintf("%d", defaultPerPage))
	params.Set("customer_id", "tavrn")
	params.Set("content_filter", "high")

	resp, err := c.http.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("klipy search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("klipy search: status %d", resp.StatusCode)
	}

	var raw struct {
		Result bool `json:"result"`
		Data   struct {
			Data []struct {
				Slug  string `json:"slug"`
				Title string `json:"title"`
				File  struct {
					Sm struct {
						Gif struct {
							URL    string `json:"url"`
							Width  int    `json:"width"`
							Height int    `json:"height"`
						} `json:"gif"`
					} `json:"sm"`
				} `json:"file"`
			} `json:"data"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("klipy parse: %w", err)
	}

	var results []KlipyResult
	for _, item := range raw.Data.Data {
		if item.File.Sm.Gif.URL == "" {
			continue
		}
		results = append(results, KlipyResult{
			Slug:   item.Slug,
			Title:  item.Title,
			URL:    item.File.Sm.Gif.URL,
			Width:  item.File.Sm.Gif.Width,
			Height: item.File.Sm.Gif.Height,
		})
	}

	return results, nil
}

// FetchGIF downloads GIF data from a URL with size limits.
func (c *KlipyClient) FetchGIF(gifURL string) ([]byte, error) {
	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Get(gifURL)
	if err != nil {
		return nil, fmt.Errorf("fetch gif: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch gif: status %d", resp.StatusCode)
	}

	// Limit read size
	limited := io.LimitReader(resp.Body, maxGIFSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read gif: %w", err)
	}
	if len(data) > maxGIFSize {
		return nil, fmt.Errorf("gif too large (>%dMB)", maxGIFSize/1024/1024)
	}

	return data, nil
}
