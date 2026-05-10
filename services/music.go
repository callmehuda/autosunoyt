package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const sunoAPIBase = "https://api.sunoapi.org/api/v1"

type MusicService struct {
	APIKey    string
	OutputDir string
}

func NewMusicService(apiKey, outputDir string) *MusicService {
	return &MusicService{APIKey: apiKey, OutputDir: outputDir}
}

// sunoapi.org request — non-custom mode (hanya butuh prompt)
type sunoGenerateRequest struct {
	CustomMode   bool   `json:"customMode"`
	Instrumental bool   `json:"instrumental"`
	Model        string `json:"model"`
	Prompt       string `json:"prompt"`
}

type sunoGenerateResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		TaskID string `json:"taskId"`
	} `json:"data"`
}

type sunoDetailResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		TaskID   string `json:"taskId"`
		Status   string `json:"status"`
		Response struct {
			SunoData []struct {
				ID       string  `json:"id"`
				AudioURL string  `json:"audioUrl"`
				Title    string  `json:"title"`
				Duration float64 `json:"duration"`
			} `json:"sunoData"`
		} `json:"response"`
		ErrorMessage string `json:"errorMessage"`
	} `json:"data"`
}

// GenerateMultiple generates songs until token budget is exhausted.
// sunoapi.org: 1 request = 10 token = 2 lagu.
// Jadi 50 token = 5 request = 10 lagu.
func (m *MusicService) GenerateMultiple(prompts []string, maxTokens, tokensPerRequest int) ([]string, error) {
	numRequests := maxTokens / tokensPerRequest
	if numRequests == 0 {
		return nil, fmt.Errorf("token budget terlalu kecil: %d < %d per request", maxTokens, tokensPerRequest)
	}

	totalSongs := numRequests * 2 // setiap request menghasilkan 2 lagu

	fmt.Printf("💡 Token budget: %d | Per request: %d | Requests: %d | Expected songs: %d\n",
		maxTokens, tokensPerRequest, numRequests, totalSongs)

	var allPaths []string

	for i := 0; i < numRequests; i++ {
		prompt := prompts[i%len(prompts)]
		fmt.Printf("\n🎵 [Request %d/%d] Prompt: %q\n", i+1, numRequests, prompt)

		// Generate — returns taskId
		taskID, err := m.submitGenerate(prompt)
		if err != nil {
			fmt.Printf("⚠️  Submit request %d failed: %v — skipping\n", i+1, err)
			continue
		}

		fmt.Printf("   📋 TaskID: %s\n", taskID)

		// Poll sampai SUCCESS
		audioURLs, err := m.pollUntilReady(taskID)
		if err != nil {
			fmt.Printf("⚠️  Poll request %d failed: %v — skipping\n", i+1, err)
			continue
		}

		// Download semua audio dari request ini (2 lagu)
		for j, url := range audioURLs {
			songIdx := i*2 + j
			dest := filepath.Join(m.OutputDir, fmt.Sprintf("song_%d.mp3", songIdx))

			fmt.Printf("   ⬇️  Downloading song %d: %s\n", songIdx+1, dest)
			if err := streamDownload(url, dest); err != nil {
				fmt.Printf("   ⚠️  Download failed: %v\n", err)
				continue
			}

			allPaths = append(allPaths, dest)
			fmt.Printf("   ✅ Song saved: %s\n", dest)
		}
	}

	if len(allPaths) == 0 {
		return nil, fmt.Errorf("semua song generation gagal")
	}

	fmt.Printf("\n🎶 Total songs generated: %d\n", len(allPaths))
	return allPaths, nil
}

func (m *MusicService) submitGenerate(prompt string) (string, error) {
	body, _ := json.Marshal(sunoGenerateRequest{
		CustomMode:   false, // Non-custom: hanya butuh prompt
		Instrumental: true,  // Lo-fi = no vocals
		Model:        "V4_5ALL",
		Prompt:       prompt,
	})

	req, err := http.NewRequest("POST", sunoAPIBase+"/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	var result sunoGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if result.Code != 200 {
		return "", fmt.Errorf("api error %d: %s", result.Code, result.Msg)
	}

	if result.Data.TaskID == "" {
		return "", fmt.Errorf("empty taskId in response")
	}

	return result.Data.TaskID, nil
}

func (m *MusicService) pollUntilReady(taskID string) ([]string, error) {
	// Stream URL siap ~30-40 detik, full download URL siap ~2-3 menit
	// Poll tiap 15 detik, max 6 menit
	maxAttempts := 24 // 24 × 15s = 360s = 6 menit

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		time.Sleep(15 * time.Second)

		req, err := http.NewRequest("GET",
			fmt.Sprintf("%s/generate/record-info?taskId=%s", sunoAPIBase, taskID),
			nil,
		)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+m.APIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("   ⚠️  Poll attempt %d failed: %v\n", attempt, err)
			continue
		}

		var detail sunoDetailResponse
		json.NewDecoder(resp.Body).Decode(&detail)
		resp.Body.Close()

		status := detail.Data.Status
		fmt.Printf("   ⏳ [%d/%d] Status: %s\n", attempt, maxAttempts, status)

		switch status {
		case "SUCCESS":
			var urls []string
			for _, track := range detail.Data.Response.SunoData {
				if track.AudioURL != "" {
					fmt.Printf("   🎵 Track ready: %s (%.1fs)\n", track.Title, track.Duration)
					urls = append(urls, track.AudioURL)
				}
			}
			if len(urls) == 0 {
				return nil, fmt.Errorf("SUCCESS tapi tidak ada audioUrl")
			}
			return urls, nil

		case "CREATE_TASK_FAILED", "GENERATE_AUDIO_FAILED", "SENSITIVE_WORD_ERROR":
			return nil, fmt.Errorf("generation failed: %s — %s", status, detail.Data.ErrorMessage)
		}
		// PENDING / TEXT_SUCCESS / FIRST_SUCCESS → lanjut polling
	}

	return nil, fmt.Errorf("timeout: taskId %s tidak selesai dalam %d detik", taskID, maxAttempts*15)
}

func streamDownload(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
