# CLAUDE.md — Attesta Frontend

> Dibaca otomatis oleh Claude Code setiap sesi di repo frontend.
> File ini berisi kontrak API antara Attesta-BE dan Attesta-FE.

---

## Konteks Sistem

**Attesta** adalah sistem verifikasi skill developer berbasis kontribusi GitHub nyata,
disimpan permanen di Monad blockchain sebagai attestation record.

Frontend bertugas:

- Menerima input dari user (GitHub username, repo, wallet address)
- Memanggil API Attesta-BE
- Menampilkan hasil analisis skill (`/analyze`) dan bukti attestation (`/attest`)

---

## Base URL

```
Development : http://localhost:4010
Production  : (diisi setelah deploy)
```

---

## Endpoints yang Tersedia

| Method | Path                     | Kegunaan                                                  |
| ------ | ------------------------ | --------------------------------------------------------- |
| GET    | `/health`                | Cek server hidup                                          |
| POST   | `/analyze`               | Analisis kontribusi GitHub → skill proof (tanpa on-chain) |
| POST   | `/attest`                | Full flow: analisis + IPFS + Monad (belum aktif)          |
| GET    | `/attestation/:id`       | Baca attestation by ID (belum aktif)                      |
| GET    | `/attestations/:address` | Semua attestation milik wallet (belum aktif)              |

> Endpoint bertanda "belum aktif" menunggu smart contract deploy di Monad testnet.

---

## GET /health

Cek apakah server backend hidup.

**Response sukses:**

```json
{
  "success": true,
  "status": "ok"
}
```

---

## POST /analyze

Endpoint utama untuk demo dan preview skill proof.
**Tidak** menyentuh blockchain atau IPFS — aman untuk dipakai di flow "preview sebelum attest".

**Request:**

```json
{
  "owner": "rizkirmdhnnn",
  "repo": "smartpres",
  "author": "GPadaka19"
}
```

| Field    | Tipe   | Wajib | Keterangan                                  |
| -------- | ------ | ----- | ------------------------------------------- |
| `owner`  | string | ✅    | GitHub username pemilik repo                |
| `repo`   | string | ✅    | Nama repo (tanpa owner prefix)              |
| `author` | string | ✅    | GitHub username kontributor yang dianalisis |

**Response sukses (200):**

```json
{
  "success": true,
  "meta": {
    "owner": "rizkirmdhnnn",
    "repo": "smartpres",
    "author": "GPadaka19",
    "commits_fetched": 9,
    "commits_analyzed": 9
  },
  "skill_proof": {
    "primary_language": "TypeScript",
    "secondary_languages": ["Python", "CSS"],
    "contribution_types": ["frontend", "backend", "devops", "config"],
    "i18n_only": false,
    "skill_tags": [
      "Next.js App Router",
      "React custom hooks",
      "Telegram Bot API",
      "session management",
      "Cloudflare Tunnel"
    ],
    "contribution_quality": "high",
    "confidence_score": 87,
    "red_flags": [],
    "summary": "Developer aktif di layer frontend Next.js dan backend API routes.",
    "period": {
      "first_commit": "2026-02-25",
      "last_commit": "2026-04-07"
    }
  }
}
```

---

## POST /attest _(belum aktif)_

Sama seperti `/analyze` tapi menyimpan hasilnya ke IPFS dan Monad blockchain.
Membutuhkan `recipient_address` (wallet Ethereum penerima attestation).

**Request:**

```json
{
  "owner": "rizkirmdhnnn",
  "repo": "smartpres",
  "author": "GPadaka19",
  "recipient_address": "0x08a45697aF8A798CB025FA073Bf90bD20D95f209"
}
```

**Response sukses (200):**

```json
{
  "success": true,
  "attestation_id": 1,
  "tx_hash": "0xabc...",
  "explorer_url": "https://testnet.monadexplorer.com/tx/0xabc...",
  "evidence_cid": "QmXyz...",
  "evidence_url": "https://QmXyz.ipfs.w3s.link",
  "skill_proof": {}
}
```

---

## Error Response (semua endpoint)

Semua error menggunakan format yang sama:

```json
{
  "success": false,
  "error": "pesan yang bisa dibaca manusia",
  "code": "ERROR_CODE"
}
```

### Daftar Error Code

| Code                    | HTTP Status | Kapan terjadi                                       |
| ----------------------- | ----------- | --------------------------------------------------- |
| `INVALID_INPUT`         | 400         | Field wajib kosong atau format salah                |
| `GITHUB_USER_NOT_FOUND` | 404         | Author tidak punya commit di repo itu               |
| `GITHUB_REPO_NOT_FOUND` | 404         | Repo tidak ditemukan atau private                   |
| `GITHUB_RATE_LIMIT`     | 429         | GitHub rate limit tercapai                          |
| `NO_VALID_COMMITS`      | 422         | Semua commit adalah merge commit atau terlalu besar |
| `AI_PARSE_ERROR`        | 500         | AI gagal return JSON valid                          |
| `IPFS_UPLOAD_FAILED`    | 500         | Upload evidence ke IPFS gagal                       |
| `CONTRACT_NOT_SET`      | 500         | Contract belum di-deploy                            |
| `TX_FAILED`             | 500         | Transaksi Monad gagal                               |
| `INTERNAL_ERROR`        | 500         | Error tidak terduga                                 |

---

## Tipe Data skill_proof

```ts
interface SkillProof {
  primary_language: string;
  secondary_languages: string[];
  contribution_types: Array<
    "frontend" | "backend" | "testing" | "docs" | "i18n" | "config" | "devops"
  >;
  i18n_only: boolean;
  skill_tags: string[]; // max 5 item
  contribution_quality: "low" | "medium" | "high";
  confidence_score: number; // 0–100
  red_flags: string[]; // array kosong jika tidak ada
  summary: string;
  period: {
    first_commit: string; // format YYYY-MM-DD
    last_commit: string; // format YYYY-MM-DD
  };
}
```

---

## Tipe Data meta (dari /analyze)

```ts
interface AnalyzeMeta {
  owner: string;
  repo: string;
  author: string;
  commits_fetched: number; // commit yang lolos filter merge
  commits_analyzed: number; // commit yang benar-benar dianalisis AI
}
```

---

## Catatan Penting untuk FE

- `success: false` selalu disertai `error` dan `code` — gunakan `code` untuk logika, `error` untuk pesan ke user
- `red_flags` bisa array kosong `[]` — render kondisional
- `secondary_languages` bisa array kosong `[]`
- `confidence_score` range 0–100, cocok untuk progress bar atau badge
- `i18n_only: true` artinya kontribusi hanya terjemahan, bukan skill teknis — bisa ditampilkan sebagai warning
- `/analyze` bisa lambat 5–15 detik (fetch GitHub + AI) — tampilkan loading state

---

## Coding Conventions BE (untuk referensi FE)

- Semua response JSON key menggunakan `snake_case`
- Boolean field tidak pernah null — selalu `true` atau `false`
- Array field tidak pernah null — bisa kosong `[]` tapi tidak `null`
