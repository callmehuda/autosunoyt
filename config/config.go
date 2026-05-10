package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	GiphyAPIKey            string // tidak dipakai lagi, tapi dibiarkan agar tidak break
	SunoAPIKey             string
	GeminiAPIKey           string
	YoutubeCredentialsFile string
	YoutubeTokenFile       string
	OutputDir              string

	// Credit math:
	// AudioRequests × 12 credits = audio cost
	// AudioRequests × 2 songs × 2 credits = MV cost
	// Total = AudioRequests × (12 + 4) = AudioRequests × 16
	// Dengan 3 requests: 3×16 = 48 credits (dari 50)
	AudioRequests int

	MusicPrompts []string
}

func Load() *Config {
	return &Config{
		SunoAPIKey:             mustEnv("SUNO_API_KEY"),
		GeminiAPIKey:           mustEnv("GEMINI_API_KEY"),
		YoutubeCredentialsFile: getOr("YOUTUBE_CREDENTIALS_FILE", "./credentials.json"),
		YoutubeTokenFile:       getOr("YOUTUBE_TOKEN_FILE", "./token.json"),
		OutputDir:              getOr("OUTPUT_DIR", "./output"),

		AudioRequests: getInt("AUDIO_REQUESTS", 3), // 3 req = 6 lagu = 48 credit

		MusicPrompts: getList("MUSIC_PROMPTS", []string{
			"chill lo-fi hip hop, rainy café, slow jazz piano",
			"lo-fi beats, cozy night, soft guitar, sleepy vibes",
			"lo-fi study music, warm vinyl crackle, ambient piano",
			"lo-fi chill, midnight city rain, mellow beats",
			"lo-fi dream, soft synth, slow bpm, peaceful",
		}),
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("missing required env: " + key)
	}
	return v
}

func getOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getList(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		return strings.Split(v, ",")
	}
	return fallback
}
