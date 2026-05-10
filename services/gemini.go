package services

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// GenerateVideoMeta generates unique title, description, and tags via Gemini.
func (g *GeminiService) GenerateVideoMeta() (*VideoMeta, error) {
	date := time.Now().Format("January 2, 2006")
	weekday := time.Now().Weekday().String()

	prompt := fmt.Sprintf(`You are a YouTube lo-fi music channel manager.
Today is %s (%s).

Generate a YouTube video metadata for a lo-fi music video.

Rules:
- Title must be UNIQUE, poetic, emotional, and feel like a diary entry or a feeling.
  Examples of style (DO NOT reuse these): 
  "Relax Your Mind 🌙", "Just a Peace Lo-Fi ☕", "It's Just a Dream ✨", "I Just Want to Sleep 😴",
  "The Rain Won't Stop 🌧️", "Nobody Knows I'm Here 🌿", "3AM and I'm Still Thinking 💭"
- Title max 60 characters including emoji
- Description must be 3-4 sentences, warm and calming in tone, first person perspective
- Tags: 8-10 relevant tags as array

Respond ONLY with raw JSON, no markdown, no backticks:
{
  "title": "...",
  "description": "...",
  "tags": ["...", "..."]
}`, date, weekday)

	reqBody, _ := json.Marshal(geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: prompt}}},
		},
	})

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=%s",
		g.APIKey,
	)

	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()

	var result geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini returned empty response")
	}

	raw := strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text)

	var meta VideoMeta
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil, fmt.Errorf("parse json: %w\nraw: %s", err, raw)
	}

	if meta.Title == "" || meta.Description == "" {
		return nil, fmt.Errorf("gemini returned incomplete metadata: %+v", meta)
	}

	fmt.Printf("🤖 Gemini generated:\n   Title: %s\n   Tags:  %v\n", meta.Title, meta.Tags)
	return &meta, nil
}
