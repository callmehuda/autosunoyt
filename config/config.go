package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	GiphyAPIKey      string
	SunoAPIKey       string
	GeminiAPIKey     string
	YoutubeTokenFile string
	OutputDir        string
	GifDir           string

	MaxTokens        int
	TokensPerRequest int
	MusicPrompts   []string
	GifKeywords    []string
	GifsPerKeyword int
}

func Load() *Config {
	return &Config{
		GiphyAPIKey:      mustEnv("GIPHY_API_KEY"),
		SunoAPIKey:       mustEnv("SUNO_API_KEY"),
		GeminiAPIKey:     mustEnv("GEMINI_API_KEY"),
		YoutubeTokenFile: getOr("YOUTUBE_TOKEN_FILE", "./token.json"),
		OutputDir:        getOr("OUTPUT_DIR", "./output"),
		GifDir:           getOr("GIF_DIR", "./gifs"),

		MaxTokens:        getInt("MAX_TOKENS", 50),
		TokensPerRequest: getInt("TOKENS_PER_REQUEST", 10),
		GifsPerKeyword: getInt("GIFS_PER_KEYWORD", 15),

		MusicPrompts: getList("MUSIC_PROMPTS", []string{
			"chill lo-fi hip hop, rainy café, slow jazz piano",
			"lo-fi beats, cozy night, soft guitar, sleepy vibes",
			"lo-fi study music, warm vinyl crackle, ambient piano",
			"lo-fi chill, midnight city rain, mellow beats",
			"lo-fi dream, soft synth, slow bpm, peaceful",
		}),

		GifKeywords: getList("GIF_KEYWORDS", []string{
			"lofi anime rain",
			"cozy cafe night anime",
			"pixel art rain city",
			"anime girl studying rain",
			"lofi night window rain",
			"anime rain window lofi",
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
