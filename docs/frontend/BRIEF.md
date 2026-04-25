# BRIEF.md — Attesta Frontend

> Konteks lengkap untuk tim frontend. Dibaca Claude Code saat butuh pemahaman lebih dalam.

---

## Gambaran Sistem

Attesta memungkinkan developer membuktikan skill mereka bukan lewat klaim, tapi lewat
kontribusi GitHub nyata yang diverifikasi AI dan disimpan permanen di blockchain Monad.

```
User input (GitHub repo + wallet)
        │
        ▼
  Attesta-BE (API)
        │
        ├─ GitHub API → ambil commit history
        ├─ AI (Claude / Llama) → analisis kontribusi
        ├─ IPFS → simpan evidence
        └─ Monad testnet → mint attestation record
        │
        ▼
  Frontend tampilkan hasil
```

---

## Alur User yang Perlu Dibuat FE

### Flow 1 — Preview (tanpa blockchain)

```
1. User isi form: GitHub owner, repo, author username
2. FE call POST /analyze
3. Tampilkan loading (bisa 5–15 detik)
4. Tampilkan skill_proof:
   - primary_language + secondary_languages
   - contribution_types sebagai badge/tag
   - skill_tags (max 5) sebagai chip
   - confidence_score sebagai progress bar atau angka
   - contribution_quality sebagai badge (low/medium/high)
   - summary sebagai paragraph
   - period.first_commit → period.last_commit sebagai timeline
   - red_flags jika ada (tampilkan sebagai warning)
   - i18n_only: true → tampilkan warning "Kontribusi mayoritas terjemahan"
```

### Flow 2 — Attest (on-chain) _(belum aktif)_

```
1. User connect wallet (MetaMask / WalletConnect)
2. User isi form: owner, repo, author, recipient_address (wallet mereka)
3. FE call POST /attest
4. Tampilkan loading (lebih lama: GitHub + AI + IPFS + Monad tx)
5. Tampilkan hasil:
   - attestation_id
   - tx_hash + link ke explorer_url
   - evidence_url (link ke IPFS)
   - skill_proof (sama seperti Flow 1)
```

---

## Data yang Diterima dari /analyze

Berikut contoh response nyata dari data test:

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
    "summary": "Developer aktif di layer frontend Next.js dan backend API routes. Melakukan refactoring besar untuk centralize session management dan CloudLab utilities. Juga setup infrastruktur Cloudflare Tunnel untuk deployment.",
    "period": {
      "first_commit": "2026-02-25",
      "last_commit": "2026-04-07"
    }
  }
}
```

---

## Saran UI per Field

| Field | Saran Tampilan |
|---|---|
| `primary_language` | Badge besar dengan warna bahasa (misal TypeScript = biru) |
| `secondary_languages` | Badge kecil, muted color |
| `contribution_types` | Chip / tag pill dengan icon per tipe |
| `skill_tags` | Highlighted chip, max 5 |
| `confidence_score` | Progress bar atau donut chart 0–100 |
| `contribution_quality` | Badge: 🟢 high · 🟡 medium · 🔴 low |
| `i18n_only: true` | Banner warning kuning: "Kontribusi mayoritas file terjemahan" |
| `red_flags` | Alert merah per item, sembunyikan section jika array kosong |
| `summary` | Paragraph teks biasa |
| `period` | "Feb 2026 – Apr 2026" atau timeline visual |
| `meta.commits_analyzed` | Subtitle kecil: "Berdasarkan 9 commit" |

---

## Penanganan Error di FE

Selalu cek `success` di response sebelum render data:

```ts
const res = await fetch("/analyze", { method: "POST", body: JSON.stringify(form) })
const data = await res.json()

if (!data.success) {
  // gunakan data.code untuk logika, data.error untuk pesan ke user
  switch (data.code) {
    case "GITHUB_USER_NOT_FOUND":
      showError("Author tidak punya commit di repo ini.")
      break
    case "GITHUB_REPO_NOT_FOUND":
      showError("Repo tidak ditemukan atau private.")
      break
    case "GITHUB_RATE_LIMIT":
      showError("GitHub sedang sibuk, coba lagi dalam beberapa menit.")
      break
    case "NO_VALID_COMMITS":
      showError("Tidak ada commit yang bisa dianalisis di repo ini.")
      break
    case "AI_PARSE_ERROR":
      showError("Analisis AI gagal, silakan coba lagi.")
      break
    default:
      showError(data.error)
  }
  return
}

// aman untuk render data.skill_proof
```

---

## Loading State

`/analyze` bisa memakan waktu **5–15 detik** karena:
1. Fetch commit list dari GitHub (~1 detik)
2. Fetch detail tiap commit, up to 20 request (~3–8 detik)
3. Kirim ke AI dan tunggu response (~2–5 detik)

Tampilkan progress step agar user tidak bingung:
```
[✓] Mengambil data GitHub...
[⟳] Menganalisis kontribusi dengan AI...
[ ] Menyiapkan hasil...
```

---

## Test Data

Gunakan data ini untuk development dan demo:

```
owner  : rizkirmdhnnn
repo   : smartpres
author : GPadaka19
```

Hasil yang diharapkan:
- `primary_language`: TypeScript
- `contribution_quality`: high
- `confidence_score`: 80–90
- `i18n_only`: false

---

## Monad Testnet (untuk Flow 2)

| Parameter | Value |
|---|---|
| Chain ID | 10143 |
| RPC | https://testnet-rpc.monad.xyz |
| Explorer | https://testnet.monadexplorer.com |
| Faucet | https://faucet.monad.xyz |

User perlu MON testnet di wallet mereka untuk bayar gas. Arahkan ke faucet jika balance kosong.

---

## Kontak & Repo

| | |
|---|---|
| Repo Backend | https://github.com/Attesta-Monad/Attesta-BE |
| Smart Contract | Repo `Attesta-SC` (belum deploy) |
