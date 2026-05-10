package services

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type VideoService struct {
	OutputDir string
}

func NewVideoService(outputDir string) *VideoService {
	return &VideoService{OutputDir: outputDir}
}

// CombineMVs concat semua MV mp4 jadi 1 video panjang + title overlay
func (v *VideoService) CombineMVs(mvPaths []string, title string) (string, error) {
	if len(mvPaths) == 0 {
		return "", fmt.Errorf("tidak ada MV untuk digabung")
	}

	fmt.Printf("🎬 Menggabungkan %d MV...\n", len(mvPaths))

	// Buat concat list
	concatFile := filepath.Join(v.OutputDir, "concat.txt")
	if err := writeConcatFile(concatFile, mvPaths); err != nil {
		return "", fmt.Errorf("buat concat file: %w", err)
	}
	defer os.Remove(concatFile)

	outPath := filepath.Join(v.OutputDir,
		fmt.Sprintf("lofi_%s.mp4", time.Now().Format("20060102_150405")))

	// Concat + title overlay
	args := []string{
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile,
		"-vf", fmt.Sprintf(
			"scale=1280:720:force_original_aspect_ratio=decrease,"+
				"pad=1280:720:(ow-iw)/2:(oh-ih)/2:color=black,"+
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

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg concat: %w", err)
	}

	// Hapus MV individual
	for _, p := range mvPaths {
		os.Remove(p)
	}

	fmt.Printf("✅ Final video: %s\n", outPath)
	return outPath, nil
}

func writeConcatFile(dest string, paths []string) error {
	var sb strings.Builder
	for _, p := range paths {
		abs, _ := filepath.Abs(p)
		sb.WriteString(fmt.Sprintf("file '%s'\n", abs))
	}
	return os.WriteFile(dest, []byte(sb.String()), 0644)
}

func escapeFfmpegText(s string) string {
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, ":", "\\:")
	return s
}
