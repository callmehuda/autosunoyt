package services

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type GifService struct {
	APIKey string
	GifDir string
}

func NewGifService(apiKey, gifDir string) *GifService {
	return &GifService{APIKey: apiKey, GifDir: gifDir}
}

type giphyResponse struct {
	Data []struct {
		ID     string `json:"id"`
		Images struct {
			Original struct {
				URL string `json:"url"`
			} `json:"original"`
		} `json:"images"`
	} `json:"data"`
}

// BulkDownload downloads GIFs for all keywords in parallel.
// Uses today's date as seed so GIF selection is different each day.
func (g *GifService) BulkDownload(keywords []string, perKeyword int) ([]string, error) {
	rng := rand.New(rand.NewSource(dateSeed()))

	// Acak urutan keywords berdasarkan hari
	shuffled := make([]string, len(keywords))
	copy(shuffled, keywords)
	rng.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	fmt.Printf("📅 Date seed: %s | Keyword order: %v\n", time.Now().Format("20060102"), shuffled)

	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		files []string
	)

	sem := make(chan struct{}, 8)

	for _, kw := range shuffled {
		urls, err := g.fetchURLs(kw, perKeyword, rng)
		if err != nil {
			fmt.Printf("⚠️  Fetch URLs for %q: %v\n", kw, err)
			continue
		}

		folder := filepath.Join(g.GifDir, sanitize(kw))
		os.MkdirAll(folder, 0755)

		for i, u := range urls {
			wg.Add(1)
			go func(u, folder string, idx int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				dest := filepath.Join(folder, fmt.Sprintf("%d.gif", idx))

				// Skip kalau sudah ada
				if _, err := os.Stat(dest); err == nil {
					mu.Lock()
					files = append(files, dest)
					mu.Unlock()
					return
				}

				if err := streamDownload(u, dest); err != nil {
					fmt.Printf("⚠️  Download failed: %v\n", err)
					return
				}

				mu.Lock()
				files = append(files, dest)
				fmt.Printf("✅ GIF: %s\n", dest)
				mu.Unlock()
			}(u, folder, i)
		}
	}

	wg.Wait()
	fmt.Printf("📦 Total GIFs downloaded: %d\n", len(files))
	return files, nil
}

func (g *GifService) fetchURLs(keyword string, limit int, rng *rand.Rand) ([]string, error) {
	var urls []string
	offset := rng.Intn(50) // Offset acak berdasarkan seed hari ini

	for len(urls) < limit {
		fetch := min(25, limit-len(urls))
		endpoint := fmt.Sprintf(
			"https://api.giphy.com/v1/gifs/search?api_key=%s&q=%s&limit=%d&offset=%d&rating=g",
			g.APIKey, url.QueryEscape(keyword), fetch, offset,
		)

		resp, err := http.Get(endpoint)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var result giphyResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		if len(result.Data) == 0 {
			break
		}

		for _, item := range result.Data {
			urls = append(urls, item.Images.Original.URL)
		}
		offset += fetch
	}

	return urls, nil
}

// AllGifs returns all downloaded GIF paths, shuffled by date seed.
func (g *GifService) AllGifs(rng *rand.Rand) ([]string, error) {
	var gifs []string

	filepath.Walk(g.GifDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".gif" {
			gifs = append(gifs, path)
		}
		return nil
	})

	if len(gifs) == 0 {
		return nil, fmt.Errorf("tidak ada GIF di %s", g.GifDir)
	}

	rng.Shuffle(len(gifs), func(i, j int) {
		gifs[i], gifs[j] = gifs[j], gifs[i]
	})

	return gifs, nil
}

func sanitize(s string) string {
	out := ""
	for _, c := range s {
		if c == ' ' {
			out += "_"
		} else {
			out += string(c)
		}
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
