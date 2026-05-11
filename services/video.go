package services

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Lo-fi fallback phrases jika Gemini gagal
var fallbackPhrases = []string{
	"enjoy the music",
	"calm and relax",
	"just keep going",
	"breathe easy",
	"let it flow",
	"find your peace",
}

type VideoService struct {
	OutputDir string
	FontPath  string
}

func NewVideoService(outputDir, fontPath string) *VideoService {
	return &VideoService{OutputDir: outputDir, FontPath: fontPath}
}

// CombineMVs: tiap MV diberi overlay phrase lo-fi → concat jadi 1 video
func (v *VideoService) CombineMVs(mvPaths []string, phrases []string) (string, error) {
	if len(mvPaths) == 0 {
		return "", fmt.Errorf("tidak ada MV untuk digabung")
	}

	// Fallback jika phrases kurang
	if len(phrases) == 0 {
		phrases = fallbackPhrases
	}

	fmt.Printf("🎬 Overlay teks + concat %d MV...\n", len(mvPaths))

	var overlaid []string
	for i, mv := range mvPaths {
		phrase := phrases[i%len(phrases)]
		out := filepath.Join(v.OutputDir, fmt.Sprintf("overlaid_%d.mp4", i))

		fmt.Printf("   [%d/%d] \"%s\"\n", i+1, len(mvPaths), phrase)
		if err := v.addOverlay(mv, out, phrase); err != nil {
			fmt.Printf("⚠️  Overlay %d failed: %v — skip\n", i+1, err)
			continue
		}
		overlaid = append(overlaid, out)
	}

	if len(overlaid) == 0 {
		return "", fmt.Errorf("semua overlay gagal")
	}

	// Concat semua segment
	concatFile := filepath.Join(v.OutputDir, "concat.txt")
	if err := writeConcatFile(concatFile, overlaid); err != nil {
		return "", err
	}
	defer os.Remove(concatFile)

	outPath := filepath.Join(v.OutputDir,
		fmt.Sprintf("lofi_%s.mp4", time.Now().Format("20060102_150405")))

	args := []string{
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile,
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
		return "", fmt.Errorf("concat: %w", err)
	}

	// Cleanup
	for _, p := range overlaid {
		os.Remove(p)
	}
	for _, p := range mvPaths {
		os.Remove(p)
	}

	fmt.Printf("✅ Final video: %s\n", outPath)
	return outPath, nil
}

// addOverlay: teks lo-fi di bawah-tengah, kecil, font Special Elite
func (v *VideoService) addOverlay(input, output, phrase string) error {
	fontOpts := ""
	if v.FontPath != "" {
		fontOpts = fmt.Sprintf("fontfile=%s:", v.FontPath)
	}

	drawtextFilter := fmt.Sprintf(
		"scale=1280:720:force_original_aspect_ratio=decrease,"+
			"pad=1280:720:(ow-iw)/2:(oh-ih)/2:color=black,"+
			"drawtext=%s"+
			"text='%s':"+
			"fontcolor=white@0.75:"+
			"fontsize=22:"+
			"x=(w-text_w)/2:y=h-th-28:"+    // bawah-tengah
			"shadowcolor=black@0.8:shadowx=1:shadowy=1",
		fontOpts,
		escapeFfmpegText(phrase),
	)

	args := []string{
		"-i", input,
		"-vf", drawtextFilter,
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",
		"-c:a", "copy",
		"-y",
		output,
	}

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
