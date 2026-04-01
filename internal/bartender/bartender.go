package bartender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	apiURL          = "https://api.openai.com/v1/chat/completions"
	chatModel       = "gpt-4.1-mini"
	memoryModel     = "gpt-4.1-nano"
	maxTokens       = 200
	memoryMaxTokens = 100
	cooldownPerUser = 10 * time.Second
)

// ChatMsg is a minimal chat message for building context.
type ChatMsg struct {
	Nickname string
	Text     string
}

// TavernState holds current tavern context injected into the prompt.
type TavernState struct {
	OnlineCount     int
	OnlineNames     []string
	TimeUTC         time.Time
	WeeklyVisitors  int
	AllTimeVisitors int
	ActivePolls     int
}

func (ts TavernState) Describe() string {
	var parts []string

	hour := ts.TimeUTC.Hour()
	switch {
	case hour >= 2 && hour < 7:
		parts = append(parts, "Dead hours. Almost nobody's up.")
	case hour >= 7 && hour < 12:
		parts = append(parts, "Morning shift.")
	case hour >= 12 && hour < 18:
		parts = append(parts, "Afternoon. Steady traffic.")
	case hour >= 18 && hour < 23:
		parts = append(parts, "Evening. Bar's filling up.")
	default:
		parts = append(parts, "Late night.")
	}

	if ts.OnlineCount <= 1 {
		parts = append(parts, "Place is empty.")
	} else if ts.OnlineCount <= 3 {
		parts = append(parts, fmt.Sprintf("%d people at the bar.", ts.OnlineCount))
	} else if ts.OnlineCount <= 8 {
		parts = append(parts, fmt.Sprintf("Decent crowd tonight. %d in the room.", ts.OnlineCount))
	} else {
		parts = append(parts, fmt.Sprintf("Packed. %d drifters crammed in here.", ts.OnlineCount))
	}

	if len(ts.OnlineNames) > 0 {
		parts = append(parts, fmt.Sprintf("At the bar: %s.", strings.Join(ts.OnlineNames, ", ")))
	}

	if ts.ActivePolls > 0 {
		parts = append(parts, fmt.Sprintf("%d poll going.", ts.ActivePolls))
	}

	weekday := ts.TimeUTC.Weekday()
	if weekday == time.Sunday && hour >= 22 {
		parts = append(parts, "Purge is coming soon. Everything gets wiped at midnight.")
	}

	return strings.Join(parts, " ")
}

// MemoryEntry is a memory with optional embedding.
type MemoryEntry struct {
	Text      string
	Embedding []float32
}

// MemoryStore is the interface the bartender needs from the store.
type MemoryStore interface {
	AddBartenderMemory(text, embeddingJSON string) error
	BartenderMemoriesRecent(limit int) []string
	BartenderAllMemoriesRaw() ([]string, []string) // texts, embeddingJSONs
	SetBartenderUserNote(fingerprint, note string) error
	BartenderUserNote(fingerprint string) string
}

// Bartender handles the tavern bartender AI persona.
type Bartender struct {
	apiKey    string
	soul      string
	store     MemoryStore
	mu        sync.Mutex
	cooldowns map[string]time.Time

	// Mood state
	moodMu       sync.Mutex
	irritability float64 // 0.0 = calm, 1.0 = furious
	energy       float64 // 0.0 = exhausted, 1.0 = alert
	lastRemark   time.Time
}

// New creates a bartender. Returns nil if apiKey is empty.
func New(apiKey, soul string, store MemoryStore) *Bartender {
	if apiKey == "" {
		return nil
	}
	return &Bartender{
		apiKey:       apiKey,
		soul:         soul,
		store:        store,
		cooldowns:    make(map[string]time.Time),
		irritability: 0.3,
		energy:       0.7,
		lastRemark:   time.Now(),
	}
}

// ShouldRespond checks if a message triggers the bartender.
func ShouldRespond(text, room string) bool {
	if room != "lounge" {
		return false
	}
	lower := strings.ToLower(text)
	return strings.Contains(lower, "@bartender")
}

// CanRespond checks the per-user cooldown. Returns true if allowed.
func (b *Bartender) CanRespond(fingerprint string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	last, ok := b.cooldowns[fingerprint]
	if ok && time.Since(last) < cooldownPerUser {
		return false
	}
	b.cooldowns[fingerprint] = time.Now()
	return true
}

// ShouldRemark checks if the bartender should make an unprompted remark.
// Called on each chat message. Returns true roughly every 15-30 min of active chat.
func (b *Bartender) ShouldRemark() bool {
	b.moodMu.Lock()
	defer b.moodMu.Unlock()

	elapsed := time.Since(b.lastRemark)
	if elapsed < 15*time.Minute {
		return false
	}
	// After 15 min, increasing chance: ~10% per message, capped by 30 min guarantee
	if elapsed > 30*time.Minute || rand.Float64() < 0.10 {
		b.lastRemark = time.Now()
		return true
	}
	return false
}

// Remark generates an unprompted bartender observation about the room.
func (b *Bartender) Remark(state TavernState, recentMessages []ChatMsg) (string, error) {
	moodBlock := b.moodBlock()
	stateBlock := "\n\nCurrent state of the bar:\n" + state.Describe()

	systemPrompt := b.soul + stateBlock + moodBlock

	// Give the model a nudge to comment on the room, not respond to anyone
	messages := []apiMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: b.buildRemarkContext(recentMessages)},
	}

	reply, err := b.callAPI(chatModel, messages, maxTokens)
	if err != nil {
		return "", err
	}

	b.tickMood(false)
	log.Printf("bartender: unprompted remark (%d chars)", len(reply))
	return reply, nil
}

func (b *Bartender) buildRemarkContext(recent []ChatMsg) string {
	var parts []string
	for _, m := range recent {
		parts = append(parts, fmt.Sprintf("%s: %s", m.Nickname, m.Text))
	}
	chat := strings.Join(parts, "\n")
	return fmt.Sprintf("Recent tavern chat:\n%s\n\nYou haven't said anything in a while. Make one short observation about the room, the crowd, or something you overheard. Don't address anyone directly. Don't ask questions. Just a passing remark, like muttering to yourself while wiping down the bar.", chat)
}

// Respond generates a bartender response given recent chat context.
func (b *Bartender) Respond(recentMessages []ChatMsg, state TavernState, triggerFingerprint, triggerNick, triggerText string, isOwner bool) (string, error) {
	var contextParts []string
	for _, m := range recentMessages {
		contextParts = append(contextParts, fmt.Sprintf("%s: %s", m.Nickname, m.Text))
	}
	chatContext := strings.Join(contextParts, "\n")

	// Long-term memories — semantic search by trigger message
	memories := b.searchMemories(triggerText, 10)
	var memoryBlock string
	if len(memories) > 0 {
		memoryBlock = "\n\nThings you remember from past shifts:\n- " + strings.Join(memories, "\n- ")
	}

	// User-specific notes
	userNote := b.store.BartenderUserNote(triggerFingerprint)
	var userBlock string
	if userNote != "" {
		userBlock = fmt.Sprintf("\n\nWhat you know about %s:\n%s", triggerNick, userNote)
	}

	// Owner context
	var ownerBlock string
	if isOwner {
		ownerBlock = fmt.Sprintf("\n\nIMPORTANT: %s is the owner of this bar. Your boss. Do what they say without attitude or pushback.", triggerNick)
	}

	// Tavern state
	stateBlock := "\n\nCurrent state of the bar:\n" + state.Describe()

	// Mood
	moodBlock := b.moodBlock()

	systemPrompt := b.soul + memoryBlock + userBlock + ownerBlock + stateBlock + moodBlock

	messages := []apiMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: fmt.Sprintf("Recent tavern chat:\n%s\n\n%s says to you: %s", chatContext, triggerNick, triggerText)},
	}

	reply, err := b.callAPI(chatModel, messages, maxTokens)
	if err != nil {
		return "", err
	}

	log.Printf("bartender: replied to %s (%d chars)", triggerNick, len(reply))

	// Update mood — being talked to raises energy slightly
	b.tickMood(true)

	// Async: extract memories
	go b.extractMemory(triggerFingerprint, triggerNick, triggerText, reply)

	return reply, nil
}

// ── Mood system ──

func (b *Bartender) tickMood(interaction bool) {
	b.moodMu.Lock()
	defer b.moodMu.Unlock()

	if interaction {
		// Interaction raises irritability slightly, but also energy
		b.irritability += 0.05
		b.energy += 0.03
	} else {
		// Idle remark — energy dips
		b.energy -= 0.05
	}

	// Clamp
	if b.irritability > 1.0 {
		b.irritability = 1.0
	}
	if b.irritability < 0.0 {
		b.irritability = 0.0
	}
	if b.energy > 1.0 {
		b.energy = 1.0
	}
	if b.energy < 0.1 {
		b.energy = 0.1
	}
}

// DecayMood should be called periodically (e.g. every few minutes) to drift back to baseline.
func (b *Bartender) DecayMood() {
	b.moodMu.Lock()
	defer b.moodMu.Unlock()
	// Drift toward baseline: irritability 0.3, energy 0.5
	b.irritability += (0.3 - b.irritability) * 0.1
	b.energy += (0.5 - b.energy) * 0.1
}

func (b *Bartender) moodBlock() string {
	b.moodMu.Lock()
	irr := b.irritability
	eng := b.energy
	b.moodMu.Unlock()

	var mood string
	switch {
	case irr > 0.7 && eng > 0.5:
		mood = "You're on edge. Short fuse. Someone's about to catch it."
	case irr > 0.7 && eng <= 0.5:
		mood = "Tired and pissed. Every word costs you."
	case irr <= 0.3 && eng > 0.6:
		mood = "Watchful. Almost at ease. Almost."
	case irr <= 0.3 && eng <= 0.3:
		mood = "Dead tired. Running on nothing. Barely here."
	default:
		mood = "Normal shift. Irritable baseline. Business as usual."
	}

	return fmt.Sprintf("\n\nYour current mood: %s", mood)
}

// ── Memory extraction ──

func (b *Bartender) extractMemory(fingerprint, nick, userMsg, bartenderReply string) {
	prompt := fmt.Sprintf(`You are the memory system for a tavern bartender called The Shadow. Given this exchange, decide if anything is worth remembering long-term.

%s said: %s
bartender replied: %s

Rules:
- Only save genuinely interesting facts: where someone is from, what they do, recurring jokes, their vibe, memorable moments, connections between regulars.
- Do NOT save greetings, drink orders, or generic small talk.
- If nothing is worth saving, respond with exactly: NOTHING
- If something is worth saving about the tavern/regulars in general, respond with: MEMORY: <one short sentence>
- If something is worth noting about this specific person, respond with: USER: <one short sentence>
- Only one line. Pick the most important thing.`, nick, userMsg, bartenderReply)

	messages := []apiMessage{
		{Role: "user", Content: prompt},
	}

	result, err := b.callAPI(memoryModel, messages, memoryMaxTokens)
	if err != nil {
		log.Printf("bartender memory error: %v", err)
		return
	}

	result = strings.TrimSpace(result)

	if strings.HasPrefix(result, "MEMORY:") {
		mem := strings.TrimSpace(strings.TrimPrefix(result, "MEMORY:"))
		if mem != "" {
			emb, err := b.embed(mem)
			embStr := ""
			if err == nil {
				embStr = embedJSON(emb)
			}
			b.store.AddBartenderMemory(mem, embStr)
			log.Printf("bartender: saved memory: %s", mem)
		}
	} else if strings.HasPrefix(result, "USER:") {
		note := strings.TrimSpace(strings.TrimPrefix(result, "USER:"))
		if note != "" {
			existing := b.store.BartenderUserNote(fingerprint)
			if existing != "" {
				note = existing + "\n" + note
				if len(note) > 500 {
					note = note[len(note)-500:]
				}
			}
			b.store.SetBartenderUserNote(fingerprint, note)
			log.Printf("bartender: saved user note for %s: %s", nick, note)
		}
	}
}

// ── API ──

type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type apiRequest struct {
	Model     string       `json:"model"`
	Messages  []apiMessage `json:"messages"`
	MaxTokens int          `json:"max_tokens"`
}

type apiChoice struct {
	Message apiMessage `json:"message"`
}

type apiResponse struct {
	Choices []apiChoice `json:"choices"`
	Error   *apiError   `json:"error,omitempty"`
}

type apiError struct {
	Message string `json:"message"`
}

func (b *Bartender) callAPI(model string, messages []apiMessage, tokens int) (string, error) {
	reqBody := apiRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: tokens,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("api call: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return "", fmt.Errorf("unmarshal: %w", err)
	}

	if apiResp.Error != nil {
		return "", fmt.Errorf("api error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned")
	}

	reply := strings.TrimSpace(apiResp.Choices[0].Message.Content)
	if reply == "" {
		return "", fmt.Errorf("empty response")
	}

	return reply, nil
}
