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

// Credit math:
//   3 audio requests × 12 credits = 36 credits → 6 songs
//   6 MV             ×  2 credits = 12 credits
//   Total = 48 credits (dari 50)

type MusicService struct {
	APIKey    string
	OutputDir string
}

func NewMusicService(apiKey, outputDir string) *MusicService {
	return &MusicService{APIKey: apiKey, OutputDir: outputDir}
}

// Song holds info about a generated track
type Song struct {
	ID       string
	AudioURL string
	Title    string
	TaskID   string // parent audio task ID (dibutuhkan untuk MV)
}

// --- Audio generation structs ---

type sunoGenerateRequest struct {
	CustomMode   bool   `json:"customMode"`
	Instrumental bool   `json:"instrumental"`
	Model        string `json:"model"`
	Prompt       string `json:"prompt"`
	CallBackUrl  string `json:"callBackUrl"`
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

// --- MV generation structs ---

type mvGenerateRequest struct {
	TaskID      string `json:"taskId"`
	AudioID     string `json:"audioId"`
	CallBackUrl string `json:"callBackUrl"`
}

type mvGenerateResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		TaskID string `json:"taskId"`
	} `json:"data"`
}

type mvDetailResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		TaskID   string `json:"taskId"`
		Status   string `json:"status"`
		Response struct {
			VideoUrl string `json:"videoUrl"`
		} `json:"response"`
		ErrorMessage string `json:"errorMessage"`
	} `json:"data"`
}

// GenerateAndMakeMV generates NUM_AUDIO_REQUESTS audio tasks (2 songs each),
// then generates MV for each song. Returns list of downloaded MP4 paths.
func (m *MusicService) GenerateAndMakeMV(prompts []string, numAudioRequests int) ([]string, error) {
	fmt.Printf("💡 Audio requests: %d → %d songs → %d credits audio + %d credits MV = %d total\n",
		numAudioRequests,
		numAudioRequests*2,
		numAudioRequests*12,
		numAudioRequests*2*2,
		numAudioRequests*12+numAudioRequests*2*2,
	)

	// Step 1: Generate semua audio
	var songs []Song
	for i := 0; i < numAudioRequests; i++ {
		prompt := prompts[i%len(prompts)]
		fmt.Printf("\n🎵 [Audio %d/%d] Prompt: %q\n", i+1, numAudioRequests, prompt)

		taskID, err := m.submitAudio(prompt)
		if err != nil {
			fmt.Printf("⚠️  Audio request %d failed: %v — skipping\n", i+1, err)
			continue
		}
		fmt.Printf("   📋 TaskID: %s\n", taskID)

		tracks, err := m.pollAudio(taskID)
		if err != nil {
			fmt.Printf("⚠️  Audio poll %d failed: %v — skipping\n", i+1, err)
			continue
		}

		for _, t := range tracks {
			t.TaskID = taskID
			songs = append(songs, t)
			fmt.Printf("   🎵 Song ready: %s (ID: %s)\n", t.Title, t.ID)
		}
	}

	if len(songs) == 0 {
		return nil, fmt.Errorf("tidak ada song yang berhasil di-generate")
	}

	fmt.Printf("\n🎶 Total songs: %d\n", len(songs))

	// Step 2: Generate MV untuk setiap song
	var mvPaths []string
	for i, song := range songs {
		fmt.Printf("\n🎬 [MV %d/%d] Song: %s\n", i+1, len(songs), song.Title)

		mvTaskID, err := m.submitMV(song.TaskID, song.ID)
		if err != nil {
			fmt.Printf("⚠️  MV submit for song %d failed: %v — skipping\n", i+1, err)
			continue
		}
		fmt.Printf("   📋 MV TaskID: %s\n", mvTaskID)

		videoURL, err := m.pollMV(mvTaskID)
		if err != nil {
			fmt.Printf("⚠️  MV poll for song %d failed: %v — skipping\n", i+1, err)
			continue
		}

		dest := filepath.Join(m.OutputDir, fmt.Sprintf("mv_%d.mp4", i))
		fmt.Printf("   ⬇️  Downloading MV: %s\n", dest)
		if err := streamDownload(videoURL, dest); err != nil {
			fmt.Printf("⚠️  MV download failed: %v\n", err)
			continue
		}

		mvPaths = append(mvPaths, dest)
		fmt.Printf("   ✅ MV saved: %s\n", dest)
	}

	if len(mvPaths) == 0 {
		return nil, fmt.Errorf("semua MV generation gagal")
	}

	fmt.Printf("\n🎞️  Total MVs downloaded: %d\n", len(mvPaths))
	return mvPaths, nil
}

func (m *MusicService) submitAudio(prompt string) (string, error) {
	body, _ := json.Marshal(sunoGenerateRequest{
		CustomMode:   false,
		Instrumental: true,
		Model:        "V4_5ALL",
		Prompt:       prompt,
		CallBackUrl:  "https://example.com/callback",
	})

	req, _ := http.NewRequest("POST", sunoAPIBase+"/generate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	var result sunoGenerateResponse
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Code != 200 {
		return "", fmt.Errorf("api error %d: %s", result.Code, result.Msg)
	}
	if result.Data.TaskID == "" {
		return "", fmt.Errorf("empty taskId")
	}
	return result.Data.TaskID, nil
}

func (m *MusicService) pollAudio(taskID string) ([]Song, error) {
	for attempt := 1; attempt <= 30; attempt++ {
		time.Sleep(15 * time.Second)

		req, _ := http.NewRequest("GET",
			fmt.Sprintf("%s/generate/record-info?taskId=%s", sunoAPIBase, taskID), nil)
		req.Header.Set("Authorization", "Bearer "+m.APIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}

		var detail sunoDetailResponse
		json.NewDecoder(resp.Body).Decode(&detail)
		resp.Body.Close()

		fmt.Printf("   ⏳ [%d/30] Status: %s\n", attempt, detail.Data.Status)

		switch detail.Data.Status {
		case "SUCCESS":
			var songs []Song
			for _, t := range detail.Data.Response.SunoData {
				if t.AudioURL != "" {
					songs = append(songs, Song{
						ID:       t.ID,
						AudioURL: t.AudioURL,
						Title:    t.Title,
					})
				}
			}
			if len(songs) == 0 {
				return nil, fmt.Errorf("SUCCESS tapi tidak ada audio")
			}
			return songs, nil
		case "CREATE_TASK_FAILED", "GENERATE_AUDIO_FAILED", "SENSITIVE_WORD_ERROR":
			return nil, fmt.Errorf("failed: %s", detail.Data.ErrorMessage)
		}
	}
	return nil, fmt.Errorf("timeout audio polling")
}

func (m *MusicService) submitMV(audioTaskID, audioID string) (string, error) {
	body, _ := json.Marshal(mvGenerateRequest{
		TaskID:      audioTaskID,
		AudioID:     audioID,
		CallBackUrl: "https://example.com/callback",
	})

	req, _ := http.NewRequest("POST", sunoAPIBase+"/mp4/generate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	var result mvGenerateResponse
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Code != 200 {
		return "", fmt.Errorf("api error %d: %s", result.Code, result.Msg)
	}
	if result.Data.TaskID == "" {
		return "", fmt.Errorf("empty mv taskId")
	}
	return result.Data.TaskID, nil
}

func (m *MusicService) pollMV(taskID string) (string, error) {
	// MV biasanya butuh 2-5 menit
	for attempt := 1; attempt <= 30; attempt++ {
		time.Sleep(15 * time.Second)

		req, _ := http.NewRequest("GET",
			fmt.Sprintf("%s/mp4/details?taskId=%s", sunoAPIBase, taskID), nil)
		req.Header.Set("Authorization", "Bearer "+m.APIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}

		var detail mvDetailResponse
		json.NewDecoder(resp.Body).Decode(&detail)
		resp.Body.Close()

		fmt.Printf("   ⏳ [%d/30] MV Status: %s\n", attempt, detail.Data.Status)

		switch detail.Data.Status {
		case "SUCCESS":
			if detail.Data.Response.VideoUrl == "" {
				return "", fmt.Errorf("SUCCESS tapi videoUrl kosong")
			}
			return detail.Data.Response.VideoUrl, nil
		case "FAILED", "ERROR":
			return "", fmt.Errorf("mv failed: %s", detail.Data.ErrorMessage)
		}
	}
	return "", fmt.Errorf("timeout MV polling")
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
