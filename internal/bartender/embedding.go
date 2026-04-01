package bartender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"time"
)

const (
	embeddingAPI   = "https://api.openai.com/v1/embeddings"
	embeddingModel = "text-embedding-3-small"
)

type embeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embeddingData struct {
	Embedding []float32 `json:"embedding"`
}

type embeddingResponse struct {
	Data  []embeddingData `json:"data"`
	Error *apiError       `json:"error,omitempty"`
}

// embed calls the OpenAI embedding API and returns the vector.
func (b *Bartender) embed(text string) ([]float32, error) {
	reqBody := embeddingRequest{
		Model: embeddingModel,
		Input: text,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest("POST", embeddingAPI, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api call: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(respBytes, &embResp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if embResp.Error != nil {
		return nil, fmt.Errorf("api error: %s", embResp.Error.Message)
	}

	if len(embResp.Data) == 0 || len(embResp.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return embResp.Data[0].Embedding, nil
}

// cosineSimilarity computes cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

type scoredMemory struct {
	text  string
	score float64
}

// searchMemories finds the top N most relevant memories for a query.
// Falls back to recent memories if embedding fails.
func (b *Bartender) searchMemories(query string, topN int) []string {
	queryEmb, err := b.embed(query)
	if err != nil {
		return b.store.BartenderMemoriesRecent(topN)
	}

	texts, embJSONs := b.store.BartenderAllMemoriesRaw()
	if len(texts) == 0 {
		return nil
	}

	var scored []scoredMemory
	var noEmb []string
	for i, t := range texts {
		if embJSONs[i] != "" {
			var emb []float32
			if json.Unmarshal([]byte(embJSONs[i]), &emb) == nil && len(emb) > 0 {
				score := cosineSimilarity(queryEmb, emb)
				scored = append(scored, scoredMemory{text: t, score: score})
				continue
			}
		}
		noEmb = append(noEmb, t)
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	var result []string
	for i := 0; i < len(scored) && len(result) < topN; i++ {
		if scored[i].score > 0.3 {
			result = append(result, scored[i].text)
		}
	}
	for i := 0; len(result) < topN && i < len(noEmb); i++ {
		result = append(result, noEmb[i])
	}

	return result
}

// embedJSON returns the embedding as a JSON string for storage.
func embedJSON(emb []float32) string {
	b, err := json.Marshal(emb)
	if err != nil {
		return ""
	}
	return string(b)
}
