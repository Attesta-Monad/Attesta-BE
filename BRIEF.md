# BRIEF.md — Attesta Backend

> Konteks lengkap project backend. Dibaca Claude Code saat butuh pemahaman lebih dalam.

---

## Konteks Project

Attesta adalah sistem verifikasi skill developer berbasis kontribusi GitHub nyata
yang tersimpan permanen di Monad sebagai attestation record — bukan NFT.

**Attesta-BE** adalah otak dari sistem ini. Semua logic analisis ada di sini:
tarik data GitHub → filter → kirim ke AI → upload IPFS → submit ke blockchain.

Repo ini berdiri sendiri (bukan monorepo). Smart contract ada di repo `Attesta-SC` terpisah.

---

## Alur Kerja Lengkap

### POST /analyze (preview, tanpa on-chain)

```
Input: { owner, repo, author }
  │
  ├─ [GitHub] GET /repos/{owner}/{repo}/languages
  │     → { "TypeScript": 123096, "Python": 5412, ... }
  │
  ├─ [GitHub] GET /repos/{owner}/{repo}/commits?author={author}&per_page=100
  │     → list commit SHA + message + date + parents
  │
  ├─ [Filter] skip jika parents.length > 1 (merge commit)
  │           ambil max 20 commit terbaru yang lolos
  │
  ├─ [GitHub] GET /repos/{owner}/{repo}/commits/{sha}  ← per commit
  │     → files changed, additions, deletions, patch
  │     → skip jika files.length > 50 (auto-generated)
  │     → potong patch ke 300 char pertama per file
  │
  ├─ [Rangkum] buat 1 payload JSON bersih untuk AI
  │
  ├─ [AI] POST ke Anthropic API
  │     → kirim payload + system prompt
  │     → terima JSON skill proof
  │     → validasi JSON, jika invalid → error AI_PARSE_ERROR
  │
  └─ Return skill proof ke client (belum ada IPFS, belum ada tx)
```

### POST /attest (full flow)

```
Input: { owner, repo, author, recipient_address }
  │
  ├─ [Sama seperti /analyze sampai dapat skill proof]
  │
  ├─ [IPFS] upload evidence package ke Web3.Storage
  │     → { meta, skill_proof, commits }
  │     → dapat CID string
  │     → jika gagal → stop, return error IPFS_UPLOAD_FAILED
  │
  ├─ [Chain] panggil contract.attest() di Monad testnet
  │     → tunggu tx confirmed
  │     → parse event AttestationCreated → dapat attestation ID
  │     → jika gagal → return error TX_FAILED
  │
  └─ Return { attestation_id, tx_hash, explorer_url, evidence_url, skill_proof }
```

---

## Data GitHub — Apa yang Diambil dan Dibuang

### Endpoint 1 — List Commits

**Yang diambil:**
```go
type CommitSummary struct {
    SHA     string    `json:"sha"`
    Message string    `json:"message"`   // dari commit.message
    Date    time.Time `json:"date"`      // dari commit.author.date
    Parents []Parent  `json:"parents"`   // untuk deteksi merge commit
}
```

**Yang dibuang:** node_id, semua *_url, author detail, committer detail, verification

### Endpoint 2 — Detail Commit

**Yang diambil:**
```go
type CommitDetail struct {
    SHA   string `json:"sha"`
    Stats struct {
        Additions int `json:"additions"`
        Deletions int `json:"deletions"`
        Total     int `json:"total"`
    } `json:"stats"`
    Files []FileChange `json:"files"`
}

type FileChange struct {
    Filename     string `json:"filename"`
    Status       string `json:"status"`     // added|modified|removed
    Additions    int    `json:"additions"`
    Deletions    int    `json:"deletions"`
    PatchPreview string `json:"patch_preview"` // 300 char pertama dari patch
}
```

**Yang dibuang:** blob_url, raw_url, contents_url, sha file, full patch

### Endpoint 3 — Repo Languages

```go
type RepoLanguages map[string]int
// contoh: { "TypeScript": 123096, "Python": 5412 }
```

---

## Filter Logic (Urutan Penting)

```
commits dari endpoint 1
        │
        ▼
[Filter 1] parents.length > 1?
    YES → skip (merge commit)
    NO  → lanjut
        │
        ▼
[Ambil max 20 terbaru yang lolos]
        │
        ▼
[Fetch endpoint 2 per SHA]
        │
        ▼
[Filter 2] files.length > 50?
    YES → skip (auto-generated / massive commit)
    NO  → lanjut
        │
        ▼
[Potong patch tiap file ke 300 char]
        │
        ▼
[Payload siap untuk AI]
```

---

## Payload ke AI — Exact Format

```json
{
  "repo": "rizkirmdhnnn/smartpres",
  "author": "GPadaka19",
  "repo_languages": {
    "TypeScript": 123096,
    "Python": 5412,
    "JavaScript": 2274,
    "CSS": 765
  },
  "commits": [
    {
      "sha": "0c7265c",
      "date": "2026-04-06T01:17:40Z",
      "message": "feat: implement authentication flow with login page and protected dashboard layout",
      "stats": {
        "additions": 37,
        "deletions": 0,
        "total": 37
      },
      "files": [
        {
          "filename": "app/dashboard/layout.tsx",
          "status": "modified",
          "additions": 30,
          "deletions": 0,
          "patch_preview": "@@ -36,6 +36,36 @@ export default function DashboardLayout({\n       return;\n     }\n     setAllowed(true);\n+\n+    let isChecking = false;\n+    const verifySession"
        }
      ]
    }
  ]
}
```

---

## System Prompt untuk Claude API

```
You are a technical contribution analyst. Analyze this developer's GitHub commits and generate a skill proof.

Rules:
- Be strict: if contributions are mostly translation files (locales/, i18n/, translations/), set i18n_only to true
- If contributions are trivial (whitespace, typo fixes only), set contribution_quality to "low"
- Infer languages from file extensions, not just repo_languages
- skill_tags must be specific and actionable (e.g. "Next.js auth" not just "frontend")
- confidence_score reflects how much data you have: few commits = lower score

Return ONLY a JSON object. No markdown. No explanation. No backticks.

Schema:
{
  "primary_language": "string",
  "secondary_languages": ["string"],
  "contribution_types": ["frontend|backend|testing|docs|i18n|config|devops"],
  "i18n_only": boolean,
  "skill_tags": ["max 5 specific skills"],
  "contribution_quality": "low|medium|high",
  "confidence_score": number (0-100),
  "red_flags": ["string"] or [],
  "summary": "2-3 sentences",
  "period": {
    "first_commit": "YYYY-MM-DD",
    "last_commit": "YYYY-MM-DD"
  }
}
```

---

## Evidence Package yang Diupload ke IPFS

```json
{
  "generated_at": "2026-04-25T10:00:00Z",
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
    "contribution_types": ["frontend", "devops"],
    "i18n_only": false,
    "skill_tags": ["Next.js", "authentication", "session management", "Telegram API", "Cloudflare"],
    "contribution_quality": "high",
    "confidence_score": 88,
    "red_flags": [],
    "summary": "Developer aktif di layer frontend Next.js dan infrastruktur.",
    "period": {
      "first_commit": "2026-02-25",
      "last_commit": "2026-04-07"
    }
  },
  "commits": [
    // semua commit detail lengkap (tanpa potongan patch)
  ]
}
```

---

## Smart Contract — Attesta.sol

Contract ada di repo `Attesta-SC` terpisah. Belum di-deploy saat dokumen ini ditulis.
ABI akan ditaruh di `abi/Attesta.json` setelah deploy.

### Function yang Dipanggil BE

```solidity
function attest(
    address recipient,
    string calldata githubUsername,
    string calldata repo,
    string calldata primaryLanguage,
    string calldata contributionType,   // join dari contribution_types dengan ", "
    bool i18nOnly,
    string[] calldata skillTags,        // max 5
    uint8 confidenceScore,              // 0-100
    uint256 commitCount,
    string calldata evidenceCid
) external onlyAuthorized returns (uint256 id)
```

### Event yang Di-parse dari Receipt

```solidity
event AttestationCreated(
    uint256 indexed id,
    address indexed attester,
    address indexed recipient,
    string repo,
    bool isVerified
)
```

### Mapping BE ke Contract

| AI Output | Contract Field |
|---|---|
| `primary_language` | `primaryLanguage` |
| `contribution_types` joined ", " | `contributionType` |
| `i18n_only` | `i18nOnly` |
| `skill_tags` | `skillTags` |
| `confidence_score` | `confidenceScore` |
| commits_analyzed count | `commitCount` |
| CID dari IPFS | `evidenceCid` |

---

## Monad Testnet

| Parameter | Value |
|---|---|
| Chain ID | 10143 |
| RPC | https://testnet-rpc.monad.xyz |
| Explorer | https://testnet.monadexplorer.com |
| Faucet | https://faucet.monad.xyz |

---

## Urutan Development yang Disarankan

```
1. Setup project: go mod init, install fiber, setup config
2. Buat main.go + router
3. GET /health  ← test dulu server jalan
4. service/github.go  ← paling krusial, test dengan curl
5. service/ai.go  ← test dengan data GitHub nyata
6. POST /analyze  ← checkpoint: 2 service sudah jalan
7. service/ipfs.go
8. service/blockchain.go  ← isi setelah CONTRACT_ADDRESS ada
9. POST /attest
10. GET /attestation/:id + GET /attestations/:address
```

---

## Test Data yang Bisa Dipakai

```bash
# Repo dan author yang sudah terbukti punya commit:
owner:  rizkirmdhnnn
repo:   smartpres
author: GPadaka19

# Hasil yang diharapkan dari /analyze:
# primary_language: TypeScript
# contribution_types: ["frontend", "devops", "backend"]
# i18n_only: false
# confidence_score: 80-95
```

---

## Referensi

| Resource | URL |
|---|---|
| Go Fiber docs | https://docs.gofiber.io |
| go-ethereum | https://geth.ethereum.org/docs |
| GitHub API | https://docs.github.com/en/rest |
| Anthropic API | https://docs.anthropic.com |
| Web3.Storage | https://web3.storage/docs |
| Monad Explorer | https://testnet.monadexplorer.com |
| Monad Faucet | https://faucet.monad.xyz |