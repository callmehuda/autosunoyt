package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"lofi-automation/config"
	"lofi-automation/services"
)

func main() {
	cfg := config.Load()

	os.MkdirAll(cfg.OutputDir, 0755)
	os.MkdirAll(cfg.GifDir, 0755)

	fmt.Printf("🎵 Lo-Fi Pipeline\n")
	fmt.Printf("📅 Date: %s\n\n", time.Now().Format("2006-01-02"))

	gifSvc := services.NewGifService(cfg.GiphyAPIKey, cfg.GifDir)
	musicSvc := services.NewMusicService(cfg.SunoAPIKey, cfg.OutputDir)
	videoSvc := services.NewVideoService(cfg.OutputDir, gifSvc)
	geminiSvc := services.NewGeminiService(cfg.GeminiAPIKey)
	ytSvc := services.NewYouTubeService(cfg.YoutubeCredentialsFile, cfg.YoutubeTokenFile)

	// Step 1: Generate judul & deskripsi via Gemini
	fmt.Println("━━━ Step 1: Generate Video Metadata (Gemini) ━━━")
	meta, err := geminiSvc.GenerateVideoMeta()
	if err != nil {
		log.Fatalf("❌ Gemini: %v", err)
	}

	// Step 2: Download GIFs
	fmt.Println("\n━━━ Step 2: Download GIFs ━━━")
	_, err = gifSvc.BulkDownload(cfg.GifKeywords, cfg.GifsPerKeyword)
	if err != nil {
		log.Fatalf("❌ GIF download: %v", err)
	}

	// Step 3: Generate songs (maks 50 token)
	fmt.Println("\n━━━ Step 3: Generate Songs ━━━")
	audioPaths, err := musicSvc.GenerateMultiple(cfg.MusicPrompts, cfg.MaxTokens, cfg.TokensPerRequest)
	if err != nil {
		log.Fatalf("❌ Music gen: %v", err)
	}

	// Step 4: Render semua lagu + GIF jadi 1 video panjang
	fmt.Println("\n━━━ Step 4: Render Combined Video ━━━")
	videoPath, err := videoSvc.RenderCombined(audioPaths, meta.Title)
	if err != nil {
		log.Fatalf("❌ Video render: %v", err)
	}

	// Step 5: Upload YouTube
	fmt.Println("\n━━━ Step 5: Upload to YouTube ━━━")
	ytURL, err := ytSvc.Upload(videoPath, meta.Title, meta.Description, meta.Tags)
	if err != nil {
		log.Fatalf("❌ YouTube upload: %v", err)
	}

	fmt.Printf("\n✅ Pipeline selesai!\n   Title: %s\n   URL:   %s\n", meta.Title, ytURL)
}
