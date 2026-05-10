package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type YouTubeService struct {
	CredentialsFile string // client_secret.json dari Google Console
	TokenFile       string // token.json hasil OAuth (berisi refresh_token)
}

func NewYouTubeService(credentialsFile, tokenFile string) *YouTubeService {
	return &YouTubeService{
		CredentialsFile: credentialsFile,
		TokenFile:       tokenFile,
	}
}

func (y *YouTubeService) Upload(videoPath, title, desc string, tags []string) (string, error) {
	ctx := context.Background()

	// Baca client credentials (client_id, client_secret)
	credBytes, err := os.ReadFile(y.CredentialsFile)
	if err != nil {
		return "", fmt.Errorf("baca credentials file %q: %w", y.CredentialsFile, err)
	}
	if len(bytes.TrimSpace(credBytes)) == 0 {
		return "", fmt.Errorf("credentials file %q kosong — pastikan secret YOUTUBE_CREDENTIALS_JSON sudah diset di GitHub", y.CredentialsFile)
	}

	config, err := google.ConfigFromJSON(credBytes, youtube.YoutubeUploadScope)
	if err != nil {
		return "", fmt.Errorf("parse credentials: %w", err)
	}

	// Baca saved OAuth token (access_token + refresh_token)
	tokenBytes, err := os.ReadFile(y.TokenFile)
	if err != nil {
		return "", fmt.Errorf("baca token: %w", err)
	}

	var tok oauth2.Token
	if err := json.Unmarshal(tokenBytes, &tok); err != nil {
		return "", fmt.Errorf("parse token: %w", err)
	}

	// TokenSource auto-refresh pakai refresh_token jika access_token expired
	tokenSource := config.TokenSource(ctx, &tok)

	svc, err := youtube.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return "", fmt.Errorf("buat youtube service: %w", err)
	}

	f, err := os.Open(videoPath)
	if err != nil {
		return "", fmt.Errorf("buka video: %w", err)
	}
	defer f.Close()

	video := &youtube.Video{
		Snippet: &youtube.VideoSnippet{
			Title:       title,
			Description: desc,
			Tags:        tags,
			CategoryId:  "10",
		},
		Status: &youtube.VideoStatus{PrivacyStatus: "public"},
	}

	call := svc.Videos.Insert([]string{"snippet", "status"}, video)
	call = call.Media(f)

	result, err := call.Do()
	if err != nil {
		return "", fmt.Errorf("upload youtube: %w", err)
	}

	return "https://youtu.be/" + result.Id, nil
}
