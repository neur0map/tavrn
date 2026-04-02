package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Result holds a single search result.
type Result struct {
	Title   string
	URL     string
	Snippet string
}

// Searcher searches the web, trying Exa first then DuckDuckGo as fallback.
type Searcher struct {
	exaKey string
	http   *http.Client
}

// New creates a searcher. If exaKey is empty, only DuckDuckGo is used.
func New(exaKey string) *Searcher {
	return &Searcher{
		exaKey: exaKey,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Search returns up to 3 results for a query.
func (s *Searcher) Search(query string) ([]Result, error) {
	if s.exaKey != "" {
		results, err := s.searchExa(query)
		if err == nil && len(results) > 0 {
			return results, nil
		}
		// Fall through to DuckDuckGo
	}
	return s.searchDDG(query)
}

func (s *Searcher) searchExa(query string) ([]Result, error) {
	body := map[string]interface{}{
		"query":      query,
		"type":       "auto",
		"numResults": 3,
		"contents": map[string]interface{}{
			"highlights": map[string]interface{}{
				"maxCharacters": 200,
				"query":         query,
			},
		},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://api.exa.ai/search", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", s.exaKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("exa: status %d", resp.StatusCode)
	}

	var raw struct {
		Results []struct {
			Title      string   `json:"title"`
			URL        string   `json:"url"`
			Highlights []string `json:"highlights"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	var results []Result
	for _, r := range raw.Results {
		snippet := ""
		if len(r.Highlights) > 0 {
			snippet = r.Highlights[0]
		}
		results = append(results, Result{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: snippet,
		})
	}
	return results, nil
}

func (s *Searcher) searchDDG(query string) ([]Result, error) {
	endpoint := "https://api.duckduckgo.com/?format=json&no_html=1&skip_disambig=1&q=" + url.QueryEscape(query)
	resp, err := s.http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var raw struct {
		AbstractText  string `json:"AbstractText"`
		AbstractURL   string `json:"AbstractURL"`
		Answer        string `json:"Answer"`
		Definition    string `json:"Definition"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var results []Result

	// Use abstract if available
	if raw.AbstractText != "" {
		results = append(results, Result{
			Title:   "Summary",
			URL:     raw.AbstractURL,
			Snippet: raw.AbstractText,
		})
	}

	// Use answer if available
	if raw.Answer != "" {
		results = append(results, Result{
			Title:   "Answer",
			Snippet: raw.Answer,
		})
	}

	// Use definition if available
	if raw.Definition != "" {
		results = append(results, Result{
			Title:   "Definition",
			Snippet: raw.Definition,
		})
	}

	// Add related topics
	for _, rt := range raw.RelatedTopics {
		if len(results) >= 3 {
			break
		}
		if rt.Text != "" {
			results = append(results, Result{
				Title:   "",
				URL:     rt.FirstURL,
				Snippet: rt.Text,
			})
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results found")
	}
	return results, nil
}

// NeedsSearch checks if a message looks like a knowledge question.
func NeedsSearch(text string) bool {
	lower := strings.ToLower(text)
	// Remove the @bartender prefix
	lower = strings.TrimPrefix(lower, "@bartender")
	lower = strings.TrimSpace(lower)

	triggers := []string{
		"what is", "what are", "what's", "whats",
		"how do", "how does", "how to", "how is",
		"why do", "why does", "why is",
		"explain", "define", "meaning of",
		"who is", "who was", "who are",
		"when did", "when was", "when is",
		"where is", "where did",
		"tell me about", "search for", "look up", "google",
	}
	for _, t := range triggers {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}

// FormatForLLM formats search results into context for the bartender.
func FormatForLLM(results []Result) string {
	var b strings.Builder
	b.WriteString("Search results (use these to answer, stay in character):\n")
	for i, r := range results {
		fmt.Fprintf(&b, "%d. %s", i+1, r.Snippet)
		if r.URL != "" {
			fmt.Fprintf(&b, " (%s)", r.URL)
		}
		b.WriteString("\n")
	}
	return b.String()
}
