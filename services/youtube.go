package services

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"
)

type YouTubeService struct {
	TokenFile string
}

func NewYouTubeService(tokenFile string) *YouTubeService {
	return &YouTubeService{TokenFile: tokenFile}
}

func (y *YouTubeService) Upload(videoPath, title, desc string, tags []string) (string, error) {
	ctx := context.Background()

	b, err := os.ReadFile(y.TokenFile)
	if err != nil {
		return "", fmt.Errorf("baca token: %w", err)
	}

	config, err := google.ConfigFromJSON(b, youtube.YoutubeUploadScope)
	if err != nil {
		return "", err
	}

	client := config.Client(ctx)
	svc, err := youtube.New(client)
	if err != nil {
		return "", err
	}

	f, err := os.Open(videoPath)
	if err != nil {
		return "", err
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
