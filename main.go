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

	fmt.Printf("🎵 Lo-Fi Pipeline\n")
	fmt.Printf("📅 Date: %s\n\n", time.Now().Format("2006-01-02"))

	musicSvc := services.NewMusicService(cfg.SunoAPIKey, cfg.OutputDir)
	videoSvc := services.NewVideoService(cfg.OutputDir)
	geminiSvc := services.NewGeminiService(cfg.GeminiAPIKey)
	ytSvc := services.NewYouTubeService(cfg.YoutubeCredentialsFile, cfg.YoutubeTokenFile)

	// Step 1: Generate judul & deskripsi via Gemini
	fmt.Println("━━━ Step 1: Generate Video Metadata (Gemini) ━━━")
	meta, err := geminiSvc.GenerateVideoMeta()
	if err != nil {
		log.Fatalf("❌ Gemini: %v", err)
	}

	// Step 2: Generate audio + MV via sunoapi.org
	// 3 audio requests × 12 cr = 36 cr → 6 lagu
	// 6 MV            ×  2 cr = 12 cr
	// Total = 48 credit
	fmt.Println("\n━━━ Step 2: Generate Songs + Music Videos ━━━")
	mvPaths, err := musicSvc.GenerateAndMakeMV(cfg.MusicPrompts, cfg.AudioRequests)
	if err != nil {
		log.Fatalf("❌ Music+MV gen: %v", err)
	}

	// Step 3: Gabung semua MV jadi 1 video panjang + title overlay
	fmt.Println("\n━━━ Step 3: Combine Music Videos ━━━")
	videoPath, err := videoSvc.CombineMVs(mvPaths, meta.Title)
	if err != nil {
		log.Fatalf("❌ Combine MVs: %v", err)
	}

	// Step 4: Upload YouTube
	fmt.Println("\n━━━ Step 4: Upload to YouTube ━━━")
	ytURL, err := ytSvc.Upload(videoPath, meta.Title, meta.Description, meta.Tags)
	if err != nil {
		log.Fatalf("❌ YouTube upload: %v", err)
	}

	fmt.Printf("\n✅ Pipeline selesai!\n   Title: %s\n   URL:   %s\n", meta.Title, ytURL)
}
