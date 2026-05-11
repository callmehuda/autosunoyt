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

const kieAPIBase = "https://api.kie.ai/api/v1"

type MusicService struct {
	APIKey    string
	OutputDir string
}

func NewMusicService(apiKey, outputDir string) *MusicService {
	return &MusicService{APIKey: apiKey, OutputDir: outputDir}
}

type Song struct {
	ID       string
	AudioURL string
	Title    string
	TaskID   string
}

// --- Audio structs ---

type audioRequest struct {
	CustomMode   bool   `json:"customMode"`
	Instrumental bool   `json:"instrumental"`
	Model        string `json:"model"`
	Prompt       string `json:"prompt"`
	CallBackUrl  string `json:"callBackUrl"`
}

type audioSubmitResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		TaskID string `json:"taskId"`
	} `json:"data"`
}

type audioDetailResp struct {
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

// --- MV structs ---

type mvRequest struct {
	TaskID      string `json:"taskId"`
	AudioID     string `json:"audioId"`
	CallBackUrl string `json:"callBackUrl"`
}

type mvSubmitResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		TaskID string `json:"taskId"`
	} `json:"data"`
}

type mvDetailResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		TaskID      string `json:"taskId"`
		SuccessFlag string `json:"successFlag"`
		Response    struct {
			VideoUrl string `json:"videoUrl"`
		} `json:"response"`
		ErrorCode    interface{} `json:"errorCode"`
		ErrorMessage interface{} `json:"errorMessage"`
	} `json:"data"`
}

// GenerateAndMakeMV:
//   numRequests × 12 cr = audio cost  (each request → 2 songs)
//   numRequests × 2 songs × 2 cr = MV cost
//   With numRequests=3: 36 + 12 = 48 cr
func (m *MusicService) GenerateAndMakeMV(prompts []string, numRequests int) ([]string, error) {
	totalSongs := numRequests * 2
	fmt.Printf("💡 Requests: %d → %d songs | Credits: %d audio + %d MV = %d total\n",
		numRequests, totalSongs,
		numRequests*12, totalSongs*2,
		numRequests*12+totalSongs*2,
	)

	// Step 1: Generate audio
	var songs []Song
	for i := 0; i < numRequests; i++ {
		prompt := prompts[i%len(prompts)]
		fmt.Printf("\n🎵 [Audio %d/%d] %q\n", i+1, numRequests, prompt)

		taskID, err := m.submitAudio(prompt)
		if err != nil {
			fmt.Printf("⚠️  Audio %d failed: %v\n", i+1, err)
			continue
		}
		fmt.Printf("   📋 TaskID: %s\n", taskID)

		tracks, err := m.pollAudio(taskID)
		if err != nil {
			fmt.Printf("⚠️  Poll %d failed: %v\n", i+1, err)
			continue
		}
		for _, t := range tracks {
			t.TaskID = taskID
			songs = append(songs, t)
			fmt.Printf("   🎵 %s (ID: %s)\n", t.Title, t.ID)
		}
	}

	if len(songs) == 0 {
		return nil, fmt.Errorf("tidak ada song yang berhasil di-generate")
	}
	fmt.Printf("\n🎶 Total songs: %d\n", len(songs))

	// Step 2: Generate MV untuk tiap song
	var mvPaths []string
	for i, song := range songs {
		fmt.Printf("\n🎬 [MV %d/%d] %s\n", i+1, len(songs), song.Title)

		mvTaskID, err := m.submitMV(song.TaskID, song.ID)
		if err != nil {
			fmt.Printf("⚠️  MV submit %d failed: %v\n", i+1, err)
			continue
		}
		fmt.Printf("   📋 MV TaskID: %s\n", mvTaskID)

		videoURL, err := m.pollMV(mvTaskID)
		if err != nil {
			fmt.Printf("⚠️  MV poll %d failed: %v\n", i+1, err)
			continue
		}

		dest := filepath.Join(m.OutputDir, fmt.Sprintf("mv_%d.mp4", i))
		if err := streamDownload(videoURL, dest); err != nil {
			fmt.Printf("⚠️  MV download %d failed: %v\n", i+1, err)
			continue
		}
		mvPaths = append(mvPaths, dest)
		fmt.Printf("   ✅ %s\n", dest)
	}

	if len(mvPaths) == 0 {
		return nil, fmt.Errorf("semua MV generation gagal")
	}
	fmt.Printf("\n🎞️  Total MVs: %d\n", len(mvPaths))
	return mvPaths, nil
}

func (m *MusicService) submitAudio(prompt string) (string, error) {
	body, _ := json.Marshal(audioRequest{
		CustomMode:   false,
		Instrumental: true,
		Model:        "V5_5",
		Prompt:       prompt,
		CallBackUrl:  "https://example.com/callback",
	})

	req, _ := http.NewRequest("POST", kieAPIBase+"/generate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var r audioSubmitResp
	json.NewDecoder(resp.Body).Decode(&r)
	if r.Code != 200 {
		return "", fmt.Errorf("api %d: %s", r.Code, r.Msg)
	}
	if r.Data.TaskID == "" {
		return "", fmt.Errorf("empty taskId")
	}
	return r.Data.TaskID, nil
}

func (m *MusicService) pollAudio(taskID string) ([]Song, error) {
	for i := 1; i <= 30; i++ {
		time.Sleep(15 * time.Second)

		req, _ := http.NewRequest("GET",
			kieAPIBase+"/generate/record-info?taskId="+taskID, nil)
		req.Header.Set("Authorization", "Bearer "+m.APIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		var r audioDetailResp
		json.NewDecoder(resp.Body).Decode(&r)
		resp.Body.Close()

		fmt.Printf("   ⏳ [%d/30] %s\n", i, r.Data.Status)

		switch r.Data.Status {
		case "SUCCESS":
			var songs []Song
			for _, t := range r.Data.Response.SunoData {
				if t.AudioURL != "" {
					songs = append(songs, Song{ID: t.ID, AudioURL: t.AudioURL, Title: t.Title})
				}
			}
			if len(songs) == 0 {
				return nil, fmt.Errorf("SUCCESS tapi audioUrl kosong")
			}
			return songs, nil
		case "CREATE_TASK_FAILED", "GENERATE_AUDIO_FAILED", "SENSITIVE_WORD_ERROR":
			return nil, fmt.Errorf("failed: %s", r.Data.ErrorMessage)
		}
	}
	return nil, fmt.Errorf("timeout audio polling")
}

func (m *MusicService) submitMV(audioTaskID, audioID string) (string, error) {
	body, _ := json.Marshal(mvRequest{
		TaskID:      audioTaskID,
		AudioID:     audioID,
		CallBackUrl: "https://example.com/callback",
	})

	req, _ := http.NewRequest("POST", kieAPIBase+"/mp4/generate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var r mvSubmitResp
	json.NewDecoder(resp.Body).Decode(&r)
	if r.Code != 200 {
		return "", fmt.Errorf("api %d: %s", r.Code, r.Msg)
	}
	if r.Data.TaskID == "" {
		return "", fmt.Errorf("empty mv taskId")
	}
	return r.Data.TaskID, nil
}

func (m *MusicService) pollMV(taskID string) (string, error) {
	for i := 1; i <= 30; i++ {
		time.Sleep(15 * time.Second)

		req, _ := http.NewRequest("GET",
			kieAPIBase+"/mp4/record-info?taskId="+taskID, nil)
		req.Header.Set("Authorization", "Bearer "+m.APIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		var r mvDetailResp
		json.NewDecoder(resp.Body).Decode(&r)
		resp.Body.Close()

		fmt.Printf("   ⏳ [%d/30] MV %s\n", i, r.Data.SuccessFlag)

		switch r.Data.SuccessFlag {
		case "SUCCESS":
			if r.Data.Response.VideoUrl == "" {
				return "", fmt.Errorf("SUCCESS tapi videoUrl kosong")
			}
			return r.Data.Response.VideoUrl, nil
		case "FAILED", "ERROR":
			return "", fmt.Errorf("mv failed: %v", r.Data.ErrorMessage)
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
