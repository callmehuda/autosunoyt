package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	Contents         []geminiContent  `json:"contents"`
	GenerationConfig *geminiGenConfig `json:"generationConfig,omitempty"`
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
	} `json:"error"`
}

var geminiModels = []string{
	"gemini-2.5-flash-preview-05-20",
	"gemini-2.5-flash",
}

// GenerateVideoMeta generates YouTube title, description, and tags.
func (g *GeminiService) GenerateVideoMeta() (*VideoMeta, error) {
	prompt := `You are a YouTube lo-fi music channel manager.

Generate YouTube video metadata for a lo-fi music video.

Rules:
- Title: UNIQUE, poetic, emotional, like a diary entry or feeling. Max 60 chars including emoji. Do NOT reference days, dates, or time of year.
  Style examples (DO NOT reuse): "Relax Your Mind 🌙", "Just a Peace Lo-Fi ☕", "It's Just a Dream ✨", "I Just Want to Sleep 😴", "The Rain Won't Stop 🌧️"
- Description: 3-4 sentences, warm and calming tone, first person perspective.
- Tags: array of 8-10 relevant strings.

Respond ONLY with valid JSON, no markdown, no backticks, no extra text:
{"title":"...","description":"...","tags":["..."]}`

	for _, model := range geminiModels {
		raw, err := g.callRaw(model, prompt)
		if err != nil {
			fmt.Printf("   ⚠️  Model %s failed: %v\n", model, err)
			continue
		}

		var meta VideoMeta
		if err := json.Unmarshal([]byte(raw), &meta); err != nil {
			fmt.Printf("   ⚠️  Parse failed: %v\n", err)
			continue
		}
		if meta.Title == "" || meta.Description == "" {
			fmt.Printf("   ⚠️  Incomplete metadata\n")
			continue
		}

		fmt.Printf("🤖 Gemini [%s]:\n   Title: %s\n   Tags:  %v\n", model, meta.Title, meta.Tags)
		return &meta, nil
	}

	return nil, fmt.Errorf("semua model gagal generate metadata")
}

// GenerateSegmentPhrases generates lo-fi overlay phrases for each video segment.
func (g *GeminiService) GenerateSegmentPhrases(count int) ([]string, error) {
	prompt := fmt.Sprintf(`You are a lo-fi music video creator.

Generate %d short phrases to display as text overlays on a lo-fi music video.

Rules:
- Each phrase is 2-5 words, all lowercase
- Calming, introspective, lo-fi vibe — like a gentle reminder or feeling
- Style examples (DO NOT reuse these exact ones):
  "enjoy the music", "calm and relax", "just keep going",
  "breathe easy", "let it flow", "find your peace",
  "close your eyes", "drift away slowly", "it's okay now"
- Each phrase must be UNIQUE
- No punctuation at the end, no emoji

Respond ONLY with a JSON array of strings, no markdown, no backticks:
["phrase one","phrase two",...]`, count)

	for _, model := range geminiModels {
		raw, err := g.callRaw(model, prompt)
		if err != nil {
			fmt.Printf("   ⚠️  Model %s failed: %v\n", model, err)
			continue
		}

		var phrases []string
		if err := json.Unmarshal([]byte(raw), &phrases); err != nil {
			fmt.Printf("   ⚠️  Parse failed: %v | raw: %s\n", err, raw)
			continue
		}
		if len(phrases) < count {
			fmt.Printf("   ⚠️  Got %d phrases, wanted %d — retrying\n", len(phrases), count)
			continue
		}

		result := phrases[:count]
		fmt.Printf("🤖 Gemini phrases: %v\n", result)
		return result, nil
	}

	return nil, fmt.Errorf("gagal generate segment phrases")
}

// callRaw calls Gemini API and returns the raw text response (stripped of markdown fences).
func (g *GeminiService) callRaw(model, prompt string) (string, error) {
	reqBody, _ := json.Marshal(geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: prompt}}},
		},
		GenerationConfig: &geminiGenConfig{
			Temperature:     0.9,
			MaxOutputTokens: 2048,
		},
	})

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		model, g.APIKey,
	)

	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	fmt.Printf("   📡 HTTP %d | preview: %.200s\n", resp.StatusCode, string(rawBody))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http %d: %s", resp.StatusCode, string(rawBody))
	}

	var result geminiResponse
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("api error %d: %s", result.Error.Code, result.Error.Message)
	}
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response (finishReason: %s)",
			func() string {
				if len(result.Candidates) > 0 {
					return result.Candidates[0].FinishReason
				}
				return "no candidates"
			}())
	}

	raw := strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	return strings.TrimSpace(raw), nil
}
