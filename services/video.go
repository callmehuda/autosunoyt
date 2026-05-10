package services

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type VideoService struct {
	OutputDir string
	GifSvc    *GifService
}

func NewVideoService(outputDir string, gifSvc *GifService) *VideoService {
	return &VideoService{OutputDir: outputDir, GifSvc: gifSvc}
}

// RenderCombined renders all songs + different GIFs into one long video.
func (v *VideoService) RenderCombined(audioPaths []string, title string) (string, error) {
	rng := rand.New(rand.NewSource(dateSeed()))

	gifs, err := v.GifSvc.AllGifs(rng)
	if err != nil {
		return "", fmt.Errorf("load gifs: %w", err)
	}

	fmt.Printf("🎞️  GIFs available: %d | Songs: %d\n", len(gifs), len(audioPaths))

	var segments []string
	for i, audioPath := range audioPaths {
		gif := gifs[i%len(gifs)]
		fmt.Printf("🎬 Rendering segment %d/%d — GIF: %s\n", i+1, len(audioPaths), filepath.Base(gif))

		segPath := filepath.Join(v.OutputDir, fmt.Sprintf("segment_%d.mp4", i))
		if err := renderSegment(gif, audioPath, segPath); err != nil {
			fmt.Printf("⚠️  Segment %d failed: %v — skipping\n", i, err)
			continue
		}
		segments = append(segments, segPath)
	}

	if len(segments) == 0 {
		return "", fmt.Errorf("semua segment gagal di-render")
	}

	concatFile := filepath.Join(v.OutputDir, "concat.txt")
	if err := writeConcatFile(concatFile, segments); err != nil {
		return "", err
	}

	outPath := filepath.Join(v.OutputDir, fmt.Sprintf("lofi_%s.mp4", time.Now().Format("20060102_150405")))
	if err := concatSegments(concatFile, outPath, title); err != nil {
		return "", fmt.Errorf("concat: %w", err)
	}

	for _, s := range segments {
		os.Remove(s)
	}
	os.Remove(concatFile)

	fmt.Printf("✅ Final video: %s\n", outPath)
	return outPath, nil
}

func renderSegment(gifPath, audioPath, outPath string) error {
	args := []string{
		"-stream_loop", "-1",
		"-i", gifPath,
		"-i", audioPath,
		"-vf", "scale=1280:720:force_original_aspect_ratio=decrease," +
			"pad=1280:720:(ow-iw)/2:(oh-ih)/2:color=black," +
			"format=yuv420p",
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "192k",
		"-shortest",
		"-y",
		outPath,
	}

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func writeConcatFile(dest string, segments []string) error {
	var sb strings.Builder
	for _, s := range segments {
		abs, _ := filepath.Abs(s)
		sb.WriteString(fmt.Sprintf("file '%s'\n", abs))
	}
	return os.WriteFile(dest, []byte(sb.String()), 0644)
}

func concatSegments(concatFile, outPath, title string) error {
	args := []string{
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile,
		"-vf", fmt.Sprintf(
			"drawtext=text='%s':fontcolor=white:fontsize=42:"+
				"x=(w-text_w)/2:y=h-th-40:"+
				"shadowcolor=black@0.8:shadowx=2:shadowy=2:"+
				"box=1:boxcolor=black@0.3:boxborderw=10",
			escapeFfmpegText(title),
		),
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "192k",
		"-y",
		outPath,
	}

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func escapeFfmpegText(s string) string {
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, ":", "\\:")
	return s
}

func dateSeed() int64 {
	dateStr := time.Now().Format("20060102")
	var seed int64
	for _, c := range dateStr {
		seed = seed*10 + int64(c-'0')
	}
	return seed
}
