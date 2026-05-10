package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type GeminiService struct {
	APIKey string
}

func NewGeminiService(apiKey string) *GeminiService {
	return &GeminiService{APIKey: apiKey}
}

type VideoMeta struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type geminiRequest struct {
	Contents         []geminiContent   `json:"contents"`
	GenerationConfig *geminiGenConfig  `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// models to try in order
var geminiModels = []string{
	"gemini-2.5-flash",
}

func (g *GeminiService) GenerateVideoMeta() (*VideoMeta, error) {
	date := time.Now().Format("January 2, 2006")
	weekday := time.Now().Weekday().String()

	prompt := fmt.Sprintf(`You are a YouTube lo-fi music channel manager.
Today is %s (%s).

Generate YouTube video metadata for a lo-fi music video.

Rules:
- Title: UNIQUE, poetic, emotional, like a diary entry or feeling. Max 60 chars including emoji.
  Style examples (DO NOT reuse): "Relax Your Mind 🌙", "Just a Peace Lo-Fi ☕", "It's Just a Dream ✨", "I Just Want to Sleep 😴", "The Rain Won't Stop 🌧️"
- Description: 3-4 sentences, warm and calming tone, first person perspective.
- Tags: array of 8-10 relevant strings.

Respond ONLY with valid JSON, no markdown, no backticks, no extra text:
{"title":"...","description":"...","tags":["..."]}`, date, weekday)

	var lastErr error
	for _, model := range geminiModels {
		meta, err := g.callAPI(model, prompt)
		if err != nil {
			fmt.Printf("   ⚠️  Model %s failed: %v\n", model, err)
			lastErr = err
			continue
		}
		fmt.Printf("🤖 Gemini [%s]:\n   Title: %s\n   Tags:  %v\n", model, meta.Title, meta.Tags)
		return meta, nil
	}

	return nil, fmt.Errorf("semua model gagal, last error: %w", lastErr)
}

func (g *GeminiService) callAPI(model, prompt string) (*VideoMeta, error) {
	reqBody, _ := json.Marshal(geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: prompt}}},
		},
		GenerationConfig: &geminiGenConfig{
			Temperature:     0.9,
			MaxOutputTokens: 512,
		},
	})

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		model, g.APIKey,
	)

	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	// Read raw body for debugging
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	fmt.Printf("   📡 HTTP %d | body preview: %.200s\n", resp.StatusCode, string(rawBody))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(rawBody))
	}

	var result geminiResponse
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("api error %d: %s", result.Error.Code, result.Error.Message)
	}

	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	candidate := result.Candidates[0]
	if len(candidate.Content.Parts) == 0 {
		return nil, fmt.Errorf("empty parts (finishReason: %s)", candidate.FinishReason)
	}

	raw := strings.TrimSpace(candidate.Content.Parts[0].Text)

	// Strip markdown code fences kalau ada
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var meta VideoMeta
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil, fmt.Errorf("parse json: %w | raw: %s", err, raw)
	}

	if meta.Title == "" || meta.Description == "" {
		return nil, fmt.Errorf("incomplete metadata: title=%q desc=%q", meta.Title, meta.Description)
	}

	return &meta, nil
}
