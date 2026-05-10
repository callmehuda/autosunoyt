# 🎵 Lo-Fi Automation

Auto-generate dan upload lo-fi music video ke YouTube setiap hari menggunakan GitHub Actions.

## Flow

```
GitHub Actions (daily 02:00 UTC / 09:00 WIB)
        │
        ▼
  Gemini API → Generate judul & deskripsi unik
        │
        ▼
  Giphy API → Download GIFs (acak berdasarkan tanggal)
        │
        ▼
  Suno API → Generate 5 lagu (50 token ÷ 10 per lagu)
        │
        ▼
  FFmpeg → Gabung semua lagu + GIF → 1 video panjang
        │
        ▼
  YouTube API → Upload otomatis
```

## Setup

### 1. Clone & siapkan secrets di GitHub

Pergi ke **Settings → Secrets and variables → Actions**, tambahkan:

| Secret | Cara Dapat |
|--------|-----------|
| `GIPHY_API_KEY` | [developers.giphy.com](https://developers.giphy.com) |
| `SUNO_API_KEY` | [suno.ai](https://suno.ai) |
| `GEMINI_API_KEY` | [aistudio.google.com](https://aistudio.google.com) |
| `YOUTUBE_TOKEN_JSON` | Lihat langkah di bawah |

### 2. Setup YouTube OAuth Token

```bash
# Install Google API client
pip install google-auth-oauthlib google-api-python-client

# Jalankan script OAuth (sekali saja)
python3 -c "
from google_auth_oauthlib.flow import InstalledAppFlow
flow = InstalledAppFlow.from_client_secrets_file('client_secret.json', ['https://www.googleapis.com/auth/youtube.upload'])
creds = flow.run_local_server(port=0)
print(creds.to_json())
"
```

Copy output JSON tersebut ke secret `YOUTUBE_TOKEN_JSON`.

### 3. Jalankan manual (opsional)

Di tab **Actions** GitHub, pilih workflow **Lo-Fi Daily Upload** → **Run workflow**.

## Environment Variables

| Var | Default | Keterangan |
|-----|---------|------------|
| `MAX_TOKENS` | `50` | Total token Suno yang dipakai |
| `TOKENS_PER_SONG` | `10` | Token per lagu |
| `GIFS_PER_KEYWORD` | `15` | Jumlah GIF per keyword |
| `OUTPUT_DIR` | `./output` | Folder output video |
| `GIF_DIR` | `./gifs` | Folder cache GIF |

## Struktur Project

```
lofi-automation/
├── main.go                        # Entry point / orchestrator
├── config/config.go               # Load env vars
├── services/
│   ├── gemini.go                  # Generate judul & deskripsi
│   ├── gif.go                     # Download GIF dari Giphy
│   ├── music.go                   # Generate musik dari Suno
│   ├── video.go                   # Render video dengan FFmpeg
│   └── youtube.go                 # Upload ke YouTube
└── .github/workflows/lofi.yml     # GitHub Actions schedule
```
