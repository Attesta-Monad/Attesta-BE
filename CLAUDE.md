# CLAUDE.md — Attesta Backend

> Dibaca otomatis oleh Claude Code setiap sesi.
> Letakkan di **root folder Attesta-BE**. Jangan hapus atau pindahkan.

---

## Project Summary

**Attesta-BE** — backend REST API untuk sistem verifikasi skill on-chain Attesta.
Menerima input GitHub username + repo, menganalisis kontribusi via AI,
mengupload evidence ke IPFS, lalu submit attestation ke smart contract di Monad testnet.

---

## Tech Stack

| Layer | Teknologi | Keterangan |
|---|---|---|
| Language | Go | versi 1.22+ |
| Framework | Go Fiber | v2 |
| Blockchain | go-ethereum (geth) | interaksi contract Monad |
| AI | Anthropic Claude API | HTTP langsung, model: claude-sonnet-4-20250514 |
| GitHub | GitHub REST API | HTTP langsung, tidak pakai SDK |
| IPFS | Web3.Storage | HTTP API langsung |

---

## File Structure

```
Attesta-BE/
├── CLAUDE.md
├── BRIEF.md
├── main.go                  ← entry point, init Fiber
├── .env                     ← JANGAN COMMIT
├── .env.example             ← safe to commit
├── .gitignore
├── go.mod
├── go.sum
│
├── config/
│   └── config.go            ← load env vars
│
├── handler/
│   ├── health.go            ← GET /health
│   ├── analyze.go           ← POST /analyze
│   ├── attest.go            ← POST /attest
│   └── attestation.go       ← GET /attestation/:id, GET /attestations/:address
│
├── service/
│   ├── github.go            ← fetch + filter commits
│   ├── ai.go                ← kirim ke Claude API, parse response
│   ├── ipfs.go              ← upload evidence ke Web3.Storage
│   └── blockchain.go        ← panggil contract via geth
│
├── model/
│   ├── github.go            ← struct untuk GitHub API response
│   ├── ai.go                ← struct untuk AI input/output
│   └── attestation.go       ← struct Attestation (mirror SC)
│
└── abi/
    └── Attesta.json         ← ABI contract (diisi setelah SC deploy)
```

---

## API Endpoints

| Method | Path | Handler | Keterangan |
|---|---|---|---|
| GET | `/health` | health.go | Health check |
| POST | `/analyze` | analyze.go | GitHub + AI, tanpa on-chain |
| POST | `/attest` | attest.go | Full flow: GitHub + AI + IPFS + Monad |
| GET | `/attestation/:id` | attestation.go | Baca dari contract by ID |
| GET | `/attestations/:address` | attestation.go | Semua attestation milik wallet |

---

## Request & Response

### POST /analyze

**Request:**
```json
{
  "owner": "rizkirmdhnnn",
  "repo": "smartpres",
  "author": "GPadaka19"
}
```

**Response:**
```json
{
  "success": true,
  "meta": {
    "owner": "rizkirmdhnnn",
    "repo": "smartpres",
    "author": "GPadaka19",
    "commits_fetched": 13,
    "commits_analyzed": 9
  },
  "skill_proof": {
    "primary_language": "TypeScript",
    "secondary_languages": ["Python", "JavaScript"],
    "contribution_types": ["frontend", "auth", "devops"],
    "i18n_only": false,
    "skill_tags": ["React", "Next.js", "authentication", "session management", "Telegram API"],
    "contribution_quality": "high",
    "confidence_score": 88,
    "red_flags": [],
    "summary": "Developer aktif berkontribusi di layer frontend dan backend.",
    "period": {
      "first_commit": "2026-02-25",
      "last_commit": "2026-04-07"
    }
  }
}
```

### POST /attest

**Request:**
```json
{
  "owner": "rizkirmdhnnn",
  "repo": "smartpres",
  "author": "GPadaka19",
  "recipient_address": "0x08a45697aF8A798CB025FA073Bf90bD20D95f209"
}
```

**Response:**
```json
{
  "success": true,
  "attestation_id": 1,
  "tx_hash": "0xabc...",
  "explorer_url": "https://testnet.monadexplorer.com/tx/0xabc...",
  "evidence_cid": "QmXyz...",
  "evidence_url": "https://QmXyz.ipfs.w3s.link",
  "skill_proof": { }
}
```

### Error Response (semua endpoint)
```json
{
  "success": false,
  "error": "pesan yang bisa dibaca manusia",
  "code": "GITHUB_USER_NOT_FOUND"
}
```

---

## Environment Variables

```bash
# Server
PORT=3000

# GitHub
GITHUB_TOKEN=ghp_xxxx          # github.com/settings/tokens → repo + read:user

# Anthropic
ANTHROPIC_API_KEY=sk-ant-xxxx  # console.anthropic.com/keys

# Web3.Storage
W3S_TOKEN=eyJxx                # console.web3.storage → Create API Token

# Monad
MONAD_RPC=https://testnet-rpc.monad.xyz
DEPLOYER_PRIVATE_KEY=0x...     # backend wallet private key
CONTRACT_ADDRESS=0x...         # diisi setelah SC deploy
```

---

## GitHub Service — Rules (WAJIB)

### 3 Endpoint yang Dipakai

```
GET /repos/{owner}/{repo}/languages
GET /repos/{owner}/{repo}/commits?author={user}&per_page=100
GET /repos/{owner}/{repo}/commits/{sha}
```

### Filter Commit

```go
// SKIP jika merge commit
if len(commit.Parents) > 1 {
    continue
}

// SKIP jika terlalu banyak file (auto-generated)
if len(detail.Files) > 50 {
    continue
}
```

### Batasan

- Max **20 commit** yang di-fetch detailnya (ambil 20 terbaru yang lolos filter)
- Patch dipotong **300 karakter** pertama per file
- Field yang dibuang sebelum kirim ke AI: `node_id`, semua `*_url`, avatar, dll

### Payload yang Dikirim ke AI

```json
{
  "repo": "owner/repo",
  "author": "username",
  "repo_languages": { "TypeScript": 123096, "Python": 5412 },
  "commits": [
    {
      "sha": "0c7265c",
      "date": "2026-04-06T01:17:40Z",
      "message": "feat: implement authentication flow...",
      "stats": { "additions": 37, "deletions": 0 },
      "files": [
        {
          "filename": "app/dashboard/layout.tsx",
          "status": "modified",
          "additions": 30,
          "deletions": 0,
          "patch_preview": "@@ -36,6 +36 ..."
        }
      ]
    }
  ]
}
```

---

## AI Service — Rules (WAJIB)

### Model
`claude-sonnet-4-20250514`, max_tokens: 1024

### Output Schema yang Diharapkan

```json
{
  "primary_language": "string",
  "secondary_languages": ["string"],
  "contribution_types": ["frontend|backend|testing|docs|i18n|config|devops"],
  "i18n_only": false,
  "skill_tags": ["max 5 item spesifik"],
  "contribution_quality": "low|medium|high",
  "confidence_score": 88,
  "red_flags": ["string atau array kosong []"],
  "summary": "2-3 kalimat",
  "period": {
    "first_commit": "YYYY-MM-DD",
    "last_commit": "YYYY-MM-DD"
  }
}
```

### Rules

- Jika Claude return bukan JSON valid → return error `AI_PARSE_ERROR`, jangan fallback
- Strip markdown fence (` ```json `) sebelum parse jika ada
- Jangan kirim raw diff lengkap ke AI, hanya `patch_preview` 300 char

---

## IPFS Service — Rules

- Upload ke `https://api.web3.storage/upload`
- Header: `Authorization: Bearer {W3S_TOKEN}`
- Body: JSON blob seluruh evidence package
- Jika upload gagal → **batalkan**, jangan lanjut ke contract
- Return CID string

### Evidence Package yang Diupload

```json
{
  "generated_at": "2026-04-25T...",
  "meta": { "owner", "repo", "author", "commits_analyzed" },
  "skill_proof": { "...full AI output..." },
  "commits": [ "...semua commit detail..." ]
}
```

---

## Blockchain Service — Rules

- Pakai `go-ethereum` untuk interaksi contract
- ABI ada di `abi/Attesta.json` (diisi setelah SC deploy)
- Panggil function `attest()` di contract
- Tunggu tx confirmed sebelum return ke handler
- Parse event `AttestationCreated` dari receipt untuk dapat attestation ID
- Jika `CONTRACT_ADDRESS` kosong di env → return error langsung, jangan panic

### Urutan di handler /attest (JANGAN diubah)

```
1. Validasi input
2. Fetch GitHub data (github service)
3. Kirim ke AI (ai service)
4. Upload ke IPFS (ipfs service)  ← jika gagal, stop di sini
5. Submit ke contract (blockchain service)
6. Return response
```

---

## Error Codes

| Code | Kondisi |
|---|---|
| `INVALID_INPUT` | Field wajib kosong atau format salah |
| `GITHUB_USER_NOT_FOUND` | Author tidak punya commit di repo |
| `GITHUB_REPO_NOT_FOUND` | Repo tidak ditemukan atau private |
| `GITHUB_RATE_LIMIT` | Rate limit GitHub tercapai |
| `NO_VALID_COMMITS` | Semua commit adalah merge commit |
| `AI_PARSE_ERROR` | Claude tidak return JSON valid |
| `IPFS_UPLOAD_FAILED` | Upload ke Web3.Storage gagal |
| `CONTRACT_NOT_SET` | CONTRACT_ADDRESS kosong di env |
| `TX_FAILED` | Transaksi Monad gagal |

---

## Coding Conventions

- Semua error di-wrap dengan konteks: `fmt.Errorf("github: fetch commits: %w", err)`
- Tidak ada `panic()` di luar main.go
- Semua HTTP call pakai timeout: 30 detik untuk GitHub/AI/IPFS, 60 detik untuk blockchain
- Log setiap step dengan prefix: `[GitHub]`, `[AI]`, `[IPFS]`, `[Chain]`
- Struct tag JSON selalu snake_case

---

## Yang DILARANG

```
❌ Hardcode API key atau private key di source code
❌ Commit file .env
❌ Lanjut ke IPFS jika AI gagal
❌ Lanjut ke contract jika IPFS gagal
❌ Fetch lebih dari 20 commit detail
❌ Kirim full patch ke AI (hanya 300 char pertama)
❌ Deploy ke Monad mainnet
```

---

## Perintah Penting

```bash
go mod init github.com/yourusername/attesta-be
go mod tidy
go run main.go

# Test endpoint
curl http://localhost:3000/health
curl -X POST http://localhost:3000/analyze \
  -H "Content-Type: application/json" \
  -d '{"owner":"rizkirmdhnnn","repo":"smartpres","author":"GPadaka19"}'
```