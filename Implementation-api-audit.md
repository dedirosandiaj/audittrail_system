# Panduan Implementasi — Auditrail (ATS) API

> Panduan tunggal untuk developer yang akan mengintegrasikan platform mereka
> dengan API **Auditrail — Audit Trail System** yang sudah di‑deploy.
>
> Base URL produksi: **`https://ats.ucentric.id`**

Dokumen ini mandiri — cukup baca file ini saja untuk menghasilkan integrasi
yang siap pakai.

---

## Apa itu Auditrail (ATS) API?

**Auditrail (Audit Trail System / ATS)** adalah API observability universal
yang dirancang sebagai **satu endpoint tunggal** untuk menampung seluruh jejak
kejadian (event) dari banyak platform dan banyak bahasa pemrograman —
web (uCuan), mobile (uKasir), backend (uPayment), dan aplikasi internal
lainnya.

### Kenapa dibuat?

Tanpa platform terpusat, setiap tim biasanya membangun sendiri:

- tabel `audit_log` di setiap database,
- stack logging terpisah (Elastic / Loki / Datadog),
- sistem error reporting terpisah (Sentry),
- pipeline metrik sendiri (Prometheus + Grafana).

Semua sistem tersebut memecah data yang sebenarnya ingin dilihat secara
bersamaan oleh tim keamanan, produk, dan operasional. **Auditrail
menggabungkan semuanya di satu tempat, dengan satu kontrak API, lewat HMAC
yang tamper‑proof.**

### Apa yang bisa dikirim?

Auditrail mengenali **lima kategori event** — semua menggunakan amplop
(envelope) JSON yang sama, hanya `category` dan isi `payload` yang berbeda:

| Kategori   | Untuk apa                                   | Contoh                                      | Disimpan di |
| ---------- | ------------------------------------------- | ------------------------------------------- | ----------- |
| `audit`    | Jejak "siapa melakukan apa" — kepatuhan     | `user.login`, `product.update`, `invoice.approved` | PostgreSQL (immutable) |
| `error`    | Exception, crash, panic                     | `exception.uncaught`, `app.panic`           | PostgreSQL + ClickHouse |
| `log`      | Log aplikasi (`logger.info`, `console.log`) | `log.info`, `log.debug`                     | ClickHouse (TTL 14 hari) |
| `metric`   | Angka/measurement                           | `api.response_time`, `queue.depth`          | ClickHouse |
| `behavior` | Interaksi pengguna                          | `page.view`, `button.click`                 | ClickHouse |

### Arsitektur singkat (big picture)

```
 uPayment · uCuan · uKasir  (bahasa apa saja)
         │  HTTPS + JSON + HMAC-SHA256
         ▼
  POST https://ats.ucentric.id/v1/events
         │
         ▼
    NATS JetStream        (buffer durable, async)
         │
    ┌────┴────┬─────────────────┐
    ▼         ▼                 ▼
 audit/      log / metric /   (worker async)
 error       behavior
    │         │
    ▼         ▼
 PostgreSQL  ClickHouse
```

Kenapa desain seperti ini?

- **Latency rendah** — client hanya menunggu `202 Accepted` dari NATS, bukan
  menunggu tulis ke database.
- **Tahan lonjakan traffic** — JetStream menyerap burst, worker drain dengan
  laju stabil.
- **Multi‑tenant** — setiap aplikasi (uPayment, uCuan, dll.) punya pasangan
  `api_key` + `secret_key` sendiri. Tenancy dipaksakan di server — satu
  aplikasi tidak bisa membaca event aplikasi lain.
- **Anti tampering** — setiap request ditandatangani HMAC‑SHA256 dengan
  `secret_key` + timestamp + nonce, sehingga tidak bisa dipalsukan atau
  di‑replay.

### Siapa yang perlu mengintegrasikan?

Semua platform / service yang ingin meninggalkan jejak. Pola tipikal:

- **Backend** (Laravel, Go, FastAPI, NestJS) — kirim event `audit` di setiap
  aksi mutasi penting (`order.created`, `invoice.approved`, dst.) dan event
  `error` dari global exception handler.
- **Mobile app** — kirim event `behavior` (tap, screen view) dan `error`
  (crash report).
- **Web frontend** — **jangan** memasukkan `secret_key` ke browser. Proxy
  lewat backend kalian sendiri, lalu backend yang menandatangani.

### Yang **bukan** tanggung jawab Auditrail

- Bukan message queue untuk komunikasi antar service (pakai NATS/Kafka
  langsung).
- Bukan APM tracing terdistribusi (walau `context.trace_id` bisa dicatat).
- Bukan gudang data untuk BI — ini event log, bukan OLAP warehouse.

---

## Daftar isi

1. [Prasyarat](#1-prasyarat)
2. [Endpoint yang di‑deploy](#2-endpoint-yang-dideploy)
3. [Autentikasi (API key + HMAC)](#3-autentikasi-api-key--hmac)
4. [Event envelope](#4-event-envelope)
5. [Resep implementasi](#5-resep-implementasi)
   - 5.1 [Node.js / TypeScript](#51-nodejs--typescript)
   - 5.2 [PHP / Laravel](#52-php--laravel)
   - 5.3 [Python / FastAPI / Django](#53-python--fastapi--django)
   - 5.4 [Go](#54-go)
   - 5.5 [cURL / shell (referensi)](#55-curl--shell-referensi)
6. [Membaca kembali event](#6-membaca-kembali-event)
7. [Penanganan error](#7-penanganan-error)
8. [Checklist produksi](#8-checklist-produksi)
9. [Verifikasi integrasi dalam 3 menit](#9-verifikasi-integrasi-dalam-3-menit)

---

## 1. Prasyarat

Anda butuh **tiga hal** sebelum menulis kode apa pun:

| Item                | Cara mendapatkan                                                       |
| ------------------- | ---------------------------------------------------------------------- |
| `AUDITRAIL_URL`     | `https://ats.ucentric.id` (fixed untuk produksi)                       |
| `AUDITRAIL_API_KEY` | Diberikan admin platform setelah panggilan `/v1/admin/applications`    |
| `AUDITRAIL_SECRET`  | Pada response yang sama — **hanya ditampilkan SEKALI**. Simpan di vault. |

Jika Anda belum punya kredensial, minta admin (pemegang `ADMIN_MASTER_TOKEN`)
untuk menjalankan:

```bash
curl -X POST https://ats.ucentric.id/v1/admin/applications \
  -H "X-Admin-Token: $ADMIN_MASTER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"code":"your-app-code","name":"Your App","type":"backend"}'
```

`type` harus salah satu dari: `web`, `mobile`, `backend`.

Simpan ketiga nilai di secret store aplikasi Anda (env vars, AWS Secrets
Manager, Vault, `.env` Laravel, dsb.). **Jangan pernah commit ke git dan
jangan pernah kirim `secret` ke browser.**

### Khusus admin — cara generate `ADMIN_MASTER_TOKEN` yang aman

`ADMIN_MASTER_TOKEN` adalah kunci master yang melindungi semua endpoint admin
(`/v1/admin/applications*`). Token ini di‑set **sekali** di tab Environment
Variables di Coolify. **Gunakan CSPRNG** — jangan pernah mengetik manual atau
mengetuk acak di keyboard.

Pilih salah satu dan tempelkan outputnya ke env var di Coolify:

```bash
# A. 64 karakter hex (256 bit) — direkomendasikan
openssl rand -hex 32

# B. Base64 URL-safe (43 karakter, 256 bit)
openssl rand -base64 32 | tr '+/' '-_' | tr -d '='

# C. Tanpa openssl
head -c 32 /dev/urandom | xxd -p -c 256
```

❌ **Token buruk — JANGAN dipakai:**
`365395937598uifhsjkhfjkshf73y8753y8rhiufhjshf`
*(panjang tapi hasil ketukan keyboard; entropy rendah, rentan brute‑force)*

✅ **Token baik — bentuk output CSPRNG (64 hex):**
`<OUTPUT_DARI__openssl_rand_hex_32__— 64_karakter_hex_acak>`

> ⚠️ **Peringatan keras.** Token di atas adalah **placeholder ilustrasi**,
> **bukan** token yang siap pakai. Jangan pernah menyalin string contoh dari
> dokumentasi (dokumen ini, Stack Overflow, tutorial, README siapa pun) dan
> memakainya sebagai rahasia produksi. Dokumen bersifat publik; begitu sebuah
> string muncul di dokumen, string itu **sudah bocor** sebelum Anda mengetiknya.
> Selalu generate token **sendiri** dengan perintah CSPRNG di atas, dan
> **jangan pernah menuliskannya di dokumen, README, commit message, atau chat**.

**Rotasi.** Jika token master dicurigai bocor (di‑paste di chat, terlihat di
screenshot, muncul di log yang dibagikan, **atau disalin dari contoh di
dokumen**), rotasi segera:

1. Generate token baru dengan salah satu perintah di atas.
2. Coolify → resource → Environment Variables → ganti `ADMIN_MASTER_TOKEN` →
   tandai sebagai **secret** → **Redeploy**.
3. Pastikan token lama sudah ditolak:
   ```bash
   curl -s -o /dev/null -w "%{http_code}\n" \
     -X GET https://ats.ucentric.id/v1/admin/applications \
     -H "X-Admin-Token: <token-lama>"     # diharapkan: 401
   ```

### Aturan penanganan rahasia (berlaku untuk `api_key`, `secret_key`, dan `ADMIN_MASTER_TOKEN`)

- **Jangan pernah paste** rahasia ke chat, email, ticket, screenshot, atau
  log yang dibagikan. Begitu keluar dari vault sekali saja, anggap sudah
  bocor.
- Simpan **hanya** di password manager / secret vault (1Password, Bitwarden,
  HCP Vault, Doppler, dsb.) — tidak di git (termasuk `.env` yang
  di‑gitignore, kecelakaan bisa terjadi).
- **Jangan pernah kirim `secret_key` ke browser atau mobile client.** Untuk
  instrumentasi di browser, proxy lewat backend kalian sendiri.
- Rahasia harus di‑generate CSPRNG (`openssl rand -hex 32`,
  `crypto.randomUUID()`, dsb.) — bukan diketik manual.
- Jika dicurigai bocor, admin rotasi:
  - kunci tenant → `POST /v1/admin/applications/{id}/rotate-key`
  - token master → update env var di Coolify + redeploy

> Prinsip emas: **"kalau manusia bisa mengetiknya, itu bukan rahasia."**

---

## 2. Endpoint yang di‑deploy

| Method | Path                                     | Fungsi                                     |
| ------ | ---------------------------------------- | ------------------------------------------ |
| POST   | `/v1/events`                             | Ingest **satu** event                      |
| POST   | `/v1/events/bulk`                        | Ingest **hingga 500** event sekaligus      |
| GET    | `/v1/events?...filters`                  | Query event milik aplikasi Anda            |
| GET    | `/healthz`                               | Liveness (tanpa auth)                      |
| GET    | `/readyz`                                | Readiness (tanpa auth)                     |

Endpoint khusus admin (bukan untuk aplikasi Anda — dipanggil oleh pemilik
platform):

| Method | Path                                                  | Fungsi                        |
| ------ | ----------------------------------------------------- | ----------------------------- |
| POST   | `/v1/admin/applications`                              | Daftarkan tenant baru         |
| POST   | `/v1/admin/applications/{id}/rotate-key`              | Rotasi api_key + secret_key   |
| DELETE | `/v1/admin/applications/{id}`                         | Cabut akses (soft delete)     |

---

## 3. Autentikasi (API key + HMAC)

**Setiap** request ke `/v1/events*` WAJIB membawa empat header berikut —
tidak ada yang opsional:

| Header          | Nilai                                                                      |
| --------------- | -------------------------------------------------------------------------- |
| `Authorization` | `Bearer <api_key>`                                                         |
| `X-Timestamp`   | **ISO 8601 / RFC 3339** (mis. `2026-05-02T12:00:00Z`). Harus dalam rentang **±5 menit** dari jam server. |
| `X-Nonce`       | String unik per request (UUID v4 disarankan). Jendela replay **10 menit**. |
| `X-Signature`   | `hmac-sha256=<lowercase-hex>` — algoritma di bawah.                        |

Plus `Content-Type: application/json` untuk POST.

### Algoritma signing (sama di semua bahasa)

```
message   = timestamp + "\n" + nonce + "\n" + raw_body
signature = HMAC_SHA256(secret, message)   // lowercase hex
header    = "hmac-sha256=" + signature
```

### Aturan yang sering bikin gagal

1. **`raw_body`** = byte sequence persis yang Anda kirim. Jangan
   men‑serialize ulang JSON setelah signing — whitespace berbeda satu
   karakter saja akan membuat signature tidak valid.
2. `X-Timestamp` adalah **RFC 3339** (mis. `2026-05-02T12:00:00Z`), sama dengan format field `timestamp` di dalam body.
3. `X-Nonce` harus unik per request. Diulang dalam 10 menit → `409`.
4. Clock skew > 5 menit → `401`. Pastikan server Anda NTP‑sync.
5. Untuk request `GET`, body yang ditandatangani adalah string kosong `""`.

---

## 4. Event envelope

Semua kategori memakai amplop yang sama — hanya `category` dan isi `payload`
yang berbeda.

```json
{
  "event_id":  "018f1d4e-8c82-7d2a-9c5a-7b3b4a0e82e2",
  "timestamp": "2026-05-02T12:00:00Z",
  "category":  "audit",
  "severity":  "info",
  "action":    "payment.transfer",
  "actor":     { "id": "u_123", "email": "budi@mail.com", "ip": "10.0.0.1" },
  "resource":  { "type": "transaction", "id": "TRX-889" },
  "payload":   { "amount": 500000, "currency": "IDR" },
  "context":   { "app": "upayment", "env": "production", "version": "1.4.2" },
  "tags":      ["checkout"],
  "status":    "success"
}
```

### Field wajib
- `timestamp` — RFC 3339.
- `category` — salah satu dari: `audit`, `error`, `log`, `metric`, `behavior`.
- `action` — huruf kecil, pola `<resource>.<verb>`, regex `^[a-z][a-z0-9]*(\.[a-z][a-z0-9_]*)+$`.

### Opsional tapi sangat dianjurkan
- `event_id` — UUID v4 yang Anda generate sendiri. Kuncinya **retry
  idempoten**.
- `severity` — default `info` jika dikosongkan.
- `actor`, `resource`, `payload`, `context`, `tags`, `status`.

### Kategori → isi `payload`

| Kategori   | `payload` tipikal                                            |
| ---------- | ------------------------------------------------------------ |
| `audit`    | snapshot `before` / `after`, `reason`, `approver`            |
| `error`    | `message`, `stack_trace`, `file`, `line`                     |
| `log`      | `message`, `logger`, field terstruktur bebas                 |
| `metric`   | field numerik, mis. `{ "endpoint": "/x", "ms": 245 }`        |
| `behavior` | `url`, `component`, `duration_ms`, dsb.                      |

---

## 5. Resep implementasi

Semua resep **siap copy‑paste** — tanpa dependency di luar standard library
bahasa masing‑masing (kecuali `uuid` jika disebut).

### 5.1 Node.js / TypeScript

Install: `npm i` (tidak ada dep — pakai `node:crypto`).

```ts
// file: audit-client.ts
import crypto, { randomUUID } from "node:crypto";

const URL    = process.env.AUDITRAIL_URL    ?? "https://ats.ucentric.id";
const KEY    = process.env.AUDITRAIL_API_KEY!;
const SECRET = process.env.AUDITRAIL_SECRET!;

export interface AuditEvent {
  category: "audit" | "error" | "log" | "metric" | "behavior";
  action:   string;                     // mis. "user.login"
  severity?: "debug" | "info" | "warn" | "error" | "critical";
  actor?:    Record<string, unknown>;
  resource?: Record<string, unknown>;
  payload?:  Record<string, unknown>;
  context?:  Record<string, unknown>;
  tags?:     string[];
  status?:   "success" | "failed";
}

export async function sendEvent(ev: AuditEvent) {
  const body = JSON.stringify({
    event_id:  randomUUID(),
    timestamp: new Date().toISOString(),
    severity:  "info",
    ...ev,
  });

  const ts    = new Date().toISOString();
  const nonce = randomUUID();
  const sig   = crypto
    .createHmac("sha256", SECRET)
    .update(`${ts}\n${nonce}\n${body}`)
    .digest("hex");

  const res = await fetch(`${URL}/v1/events`, {
    method: "POST",
    headers: {
      "Authorization": `Bearer ${KEY}`,
      "X-Timestamp":   ts,
      "X-Nonce":       nonce,
      "X-Signature":   `hmac-sha256=${sig}`,
      "Content-Type":  "application/json",
    },
    body,
  });
  if (!res.ok) throw new Error(`auditrail ${res.status}: ${await res.text()}`);
  return res.json();
}
```

Contoh pemakaian:

```ts
await sendEvent({
  category: "audit",
  action:   "order.created",
  actor:    { id: "u_42",   type: "user"  },
  resource: { id: "ord_91", type: "order" },
  payload:  { amount: 125000, currency: "IDR" },
});
```

**Pola async untuk Express / Nest / Fastify:**

```ts
app.post("/checkout", async (req, res) => {
  const result = await doCheckout(req.body);
  res.json(result);
  // fire-and-forget — jangan pernah memblokir response HTTP
  sendEvent({
    category: "audit",
    action:   "order.checkout",
    actor:    { id: req.user.id },
    resource: { id: result.orderId, type: "order" },
    payload:  { total: result.total },
  }).catch(err => console.error("auditrail failed:", err));
});
```

### 5.2 PHP / Laravel

Tanpa package Composer — PHP 8+ murni.

```php
<?php
// app/Services/Auditrail.php
namespace App\Services;

use RuntimeException;

final class Auditrail
{
    public function __construct(
        private string $apiKey  = '',
        private string $secret  = '',
        private string $baseUrl = 'https://ats.ucentric.id',
    ) {
        $this->apiKey ||= env('AUDITRAIL_API_KEY');
        $this->secret ||= env('AUDITRAIL_SECRET');
        $this->baseUrl = env('AUDITRAIL_URL', $this->baseUrl);
    }

    public function send(array $event): array
    {
        $event['event_id']  ??= $this->uuid();
        $event['timestamp'] ??= gmdate('Y-m-d\TH:i:s\Z');
        $event['severity']  ??= 'info';

        $body  = json_encode($event, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
        $ts    = gmdate('Y-m-d\TH:i:s\Z');
        $nonce = $this->uuid();
        $sig   = hash_hmac('sha256', "$ts\n$nonce\n$body", $this->secret);

        $ch = curl_init("{$this->baseUrl}/v1/events");
        curl_setopt_array($ch, [
            CURLOPT_POST           => true,
            CURLOPT_POSTFIELDS     => $body,
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_TIMEOUT        => 10,
            CURLOPT_HTTPHEADER     => [
                "Authorization: Bearer {$this->apiKey}",
                "X-Timestamp: $ts",
                "X-Nonce: $nonce",
                "X-Signature: hmac-sha256=$sig",
                "Content-Type: application/json",
            ],
        ]);
        $resp = curl_exec($ch);
        $code = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        curl_close($ch);

        if ($code >= 400) {
            throw new RuntimeException("auditrail $code: $resp");
        }
        return json_decode($resp, true) ?? [];
    }

    private function uuid(): string
    {
        $d = random_bytes(16);
        $d[6] = chr((ord($d[6]) & 0x0f) | 0x40);
        $d[8] = chr((ord($d[8]) & 0x3f) | 0x80);
        return vsprintf('%s%s-%s-%s-%s-%s%s%s', str_split(bin2hex($d), 4));
    }
}
```

`.env`:

```
AUDITRAIL_URL=https://ats.ucentric.id
AUDITRAIL_API_KEY=ak_live_xxx
AUDITRAIL_SECRET=sk_live_xxx
```

Pemakaian di controller (dispatch ke queue untuk async):

```php
// app/Jobs/SendAuditEvent.php
class SendAuditEvent implements ShouldQueue
{
    public function __construct(public array $event) {}
    public function handle(Auditrail $audit): void { $audit->send($this->event); }
}

// Di manapun di aplikasi Anda:
SendAuditEvent::dispatch([
    'category' => 'audit',
    'action'   => 'invoice.created',
    'actor'    => ['id' => auth()->id(), 'type' => 'user'],
    'resource' => ['id' => $invoice->id, 'type' => 'invoice'],
    'payload'  => ['total' => $invoice->total],
]);
```

Global exception reporter (`app/Exceptions/Handler.php`):

```php
public function report(\Throwable $e): void
{
    SendAuditEvent::dispatch([
        'category' => 'error',
        'severity' => 'error',
        'action'   => 'app.exception',
        'payload'  => [
            'message' => $e->getMessage(),
            'file'    => $e->getFile(),
            'line'    => $e->getLine(),
            'trace'   => $e->getTraceAsString(),
        ],
    ]);
    parent::report($e);
}
```

### 5.3 Python / FastAPI / Django

Hanya stdlib. Python 3.9+.

```python
# auditrail.py
import hashlib, hmac, json, os, time, urllib.request, uuid
from datetime import datetime, timezone

URL    = os.environ.get("AUDITRAIL_URL", "https://ats.ucentric.id")
KEY    = os.environ["AUDITRAIL_API_KEY"]
SECRET = os.environ["AUDITRAIL_SECRET"].encode()


def send(event: dict) -> dict:
    event.setdefault("event_id",  str(uuid.uuid4()))
    event.setdefault("timestamp", datetime.now(tz=timezone.utc)
                                  .isoformat(timespec="seconds").replace("+00:00", "Z"))
    event.setdefault("severity",  "info")

    body  = json.dumps(event, separators=(",", ":")).encode()
    ts    = datetime.now(tz=timezone.utc).isoformat(timespec="seconds").replace("+00:00", "Z")
    nonce = str(uuid.uuid4())
    sig   = hmac.new(SECRET, f"{ts}\n{nonce}\n".encode() + body, hashlib.sha256).hexdigest()

    req = urllib.request.Request(
        f"{URL}/v1/events",
        data=body, method="POST",
        headers={
            "Authorization": f"Bearer {KEY}",
            "X-Timestamp":   ts,
            "X-Nonce":       nonce,
            "X-Signature":   f"hmac-sha256={sig}",
            "Content-Type":  "application/json",
        },
    )
    with urllib.request.urlopen(req, timeout=10) as r:
        return json.loads(r.read())
```

Middleware FastAPI:

```python
from fastapi import FastAPI, Request
from auditrail import send
import asyncio

app = FastAPI()

@app.middleware("http")
async def audit_mw(request: Request, call_next):
    resp = await call_next(request)
    asyncio.get_event_loop().run_in_executor(None, send, {
        "category": "behavior",
        "action":   "http.request",
        "payload":  {"method": request.method,
                     "path":   request.url.path,
                     "status": resp.status_code},
    })
    return resp
```

Django logging handler:

```python
import logging
class AuditrailHandler(logging.Handler):
    def emit(self, record):
        try:
            send({
                "category": "log",
                "severity": record.levelname.lower(),
                "action":   "app.log",
                "payload":  {"message": self.format(record),
                             "logger":  record.name},
            })
        except Exception:
            self.handleError(record)
```

### 5.4 Go

Hanya stdlib (kecuali `github.com/google/uuid`).

```go
// auditrail/client.go
package auditrail

import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strconv"
    "time"

    "github.com/google/uuid"
)

type Client struct {
    APIKey, Secret, BaseURL string
    HTTP                    *http.Client
}

type Event struct {
    EventID   string         `json:"event_id,omitempty"`
    Timestamp string         `json:"timestamp,omitempty"`
    Category  string         `json:"category"`
    Severity  string         `json:"severity,omitempty"`
    Action    string         `json:"action"`
    Actor     map[string]any `json:"actor,omitempty"`
    Resource  map[string]any `json:"resource,omitempty"`
    Payload   map[string]any `json:"payload,omitempty"`
    Context   map[string]any `json:"context,omitempty"`
    Tags      []string       `json:"tags,omitempty"`
    Status    string         `json:"status,omitempty"`
}

func (c *Client) Send(e Event) error {
    if e.EventID   == "" { e.EventID   = uuid.NewString() }
    if e.Timestamp == "" { e.Timestamp = time.Now().UTC().Format(time.RFC3339) }
    if e.Severity  == "" { e.Severity  = "info" }

    body, _ := json.Marshal(e)
    ts    := time.Now().UTC().Format(time.RFC3339)
    nonce := uuid.NewString()

    mac := hmac.New(sha256.New, []byte(c.Secret))
    fmt.Fprintf(mac, "%s\n%s\n", ts, nonce)
    mac.Write(body)
    sig := hex.EncodeToString(mac.Sum(nil))

    req, _ := http.NewRequest("POST", c.BaseURL+"/v1/events", bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+c.APIKey)
    req.Header.Set("X-Timestamp",   ts)
    req.Header.Set("X-Nonce",       nonce)
    req.Header.Set("X-Signature",   "hmac-sha256="+sig)
    req.Header.Set("Content-Type",  "application/json")

    h := c.HTTP
    if h == nil { h = http.DefaultClient }
    resp, err := h.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        b, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("auditrail %d: %s", resp.StatusCode, b)
    }
    return nil
}
```

Pemakaian:

```go
c := &auditrail.Client{
    APIKey:  os.Getenv("AUDITRAIL_API_KEY"),
    Secret:  os.Getenv("AUDITRAIL_SECRET"),
    BaseURL: "https://ats.ucentric.id",
}

go c.Send(auditrail.Event{    // non-blocking
    Category: "audit",
    Action:   "user.created",
    Actor:    map[string]any{"id": "admin_1", "type": "admin"},
    Resource: map[string]any{"id": "u_999",   "type": "user"},
})
```

### 5.5 cURL / shell (referensi)

Gunakan ini untuk sanity‑check kredensial dan konektivitas sebelum menulis
kode sungguhan:

```bash
#!/usr/bin/env bash
set -euo pipefail
: "${AUDITRAIL_API_KEY:?}"; : "${AUDITRAIL_SECRET:?}"

URL="https://ats.ucentric.id"
BODY='{"timestamp":"'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'","category":"audit","action":"smoke.test","actor":{"id":"dev"}}'
TS=$(date -u +%Y-%m-%dT%H:%M:%SZ)
NONCE=$(uuidgen | tr A-Z a-z)
SIG=$(printf '%s\n%s\n%s' "$TS" "$NONCE" "$BODY" \
      | openssl dgst -sha256 -hmac "$AUDITRAIL_SECRET" -hex | awk '{print $2}')

curl -sS -X POST "$URL/v1/events" \
  -H "Authorization: Bearer $AUDITRAIL_API_KEY" \
  -H "X-Timestamp: $TS" \
  -H "X-Nonce: $NONCE" \
  -H "X-Signature: hmac-sha256=$SIG" \
  -H "Content-Type: application/json" \
  --data "$BODY"
```

Diharapkan: `HTTP 202` dan body JSON berisi
`{"accepted":true,"event_ids":[...],"count":1}`.

---

## 6. Membaca kembali event

API key Anda hanya bisa membaca event milik **aplikasi sendiri** — tenancy
dipaksakan di sisi server.

Filter yang didukung (semua opsional, bisa dikombinasikan):

| Query param     | Arti                                         |
| --------------- | -------------------------------------------- |
| `actor_id`      | `actor.id` cocok persis                      |
| `action`        | `action` cocok persis (mis. `user.login`)    |
| `resource_type` | `resource.type`                              |
| `resource_id`   | `resource.id`                                |
| `status`        | `success` atau `failed`                      |
| `from`          | Batas bawah RFC 3339 (inklusif)              |
| `to`            | Batas atas RFC 3339 (inklusif)               |
| `after_id`      | Cursor — kembalikan baris dengan id > after_id |
| `limit`         | 1..500, default 50                           |

Ingat: request `GET` **tetap butuh keempat header auth**, ditandatangani
dengan body kosong (`""`).

Contoh Node.js:

```ts
async function listEvents(params: Record<string, string>) {
  const qs    = new URLSearchParams(params).toString();
  const body  = "";
  const ts    = new Date().toISOString();
  const nonce = randomUUID();
  const sig   = crypto.createHmac("sha256", SECRET)
                      .update(`${ts}\n${nonce}\n${body}`)
                      .digest("hex");

  const res = await fetch(`${URL}/v1/events?${qs}`, {
    headers: {
      "Authorization": `Bearer ${KEY}`,
      "X-Timestamp":   ts,
      "X-Nonce":       nonce,
      "X-Signature":   `hmac-sha256=${sig}`,
    },
  });
  return res.json();
}

await listEvents({ action: "user.login", limit: "100" });
```

---

## 7. Penanganan error

| HTTP | Arti                                                                 | Retry? |
| ---- | -------------------------------------------------------------------- | ------ |
| 202  | Diterima — event antre untuk disimpan durable.                       | —      |
| 400  | JSON cacat / field wajib hilang, atau **X-Timestamp bukan RFC 3339**. | Tidak — perbaiki format timestamp. |
| 401  | API key / signature tidak valid, atau `X-Timestamp` skew > 5 menit.  | Tidak. Cek secret & NTP. |
| 409  | Nonce diulang — `X-Nonce` yang sama dipakai dua kali dalam 10 menit. | Tidak — selalu UUID baru. |
| 422  | Validasi skema gagal (response body berisi array `errors[]`).        | Tidak — perbaiki payload. |
| 429  | Rate limit kategori terlampaui.                                      | Ya — back off. |
| 5xx  | Error transient di server. Event mungkin tetap ter‑ingest.           | Ya — exponential backoff, maks 3 percobaan. |

**Kebijakan retry (disarankan):**
- Retry hanya untuk network error dan `5xx` / `429`.
- Jeda: 250 ms → 500 ms → 1 s → berhenti.
- **Pertahankan `event_id` yang sama** di setiap retry — server
  men‑deduplikasi berdasarkan `event_id`, jadi retry untuk panggilan yang
  sebenarnya sukses tapi timeout di jaringan tetap aman.
- Generate **`X-Nonce` dan `X-Timestamp` baru** untuk setiap retry
  (signature harus dihitung ulang).

---

## 8. Checklist produksi

Centang semua sebelum menyatakan integrasi selesai:

- [ ] Kredensial disimpan di secret manager, bukan `.env` yang di‑commit ke git.
- [ ] `secret_key` tidak pernah terekspos ke browser atau mobile client.
- [ ] `send()` dipanggil di luar jalur kritis request (queue, goroutine,
      background task, atau `runInExecutor`).
- [ ] Jam host sudah NTP‑sync.
- [ ] Setiap event punya `event_id` yang di‑generate klien (UUID v4).
- [ ] Retry mempertahankan `event_id` tapi regenerate `X-Nonce` + `X-Timestamp`.
- [ ] Jalur dengan volume tinggi pakai `/v1/events/bulk` (flush tiap 2 detik
      atau tiap 200 event, mana yang lebih cepat).
- [ ] Error dari `send()` di‑log tapi **tidak menggagalkan aksi user**.
- [ ] Aplikasi (kunci) terpisah untuk `staging` dan `production`.
- [ ] Admin punya runbook untuk `POST /v1/admin/applications/{id}/rotate-key`
      kalau kunci bocor.

---

## 9. Verifikasi integrasi dalam 3 menit

```bash
# 1. Service hidup
curl -sS https://ats.ucentric.id/healthz          # => ok
curl -sS https://ats.ucentric.id/readyz           # => ready

# 2. Kirim event smoke-test (pakai script shell di §5.5)
AUDITRAIL_API_KEY=ak_live_xxx \
AUDITRAIL_SECRET=sk_live_xxx \
./smoke.sh                                         # => HTTP 202

# 3. Baca kembali
TS=$(date +%s); NONCE=$(uuidgen | tr A-Z a-z)
SIG=$(printf '%s\n%s\n' "$TS" "$NONCE" \
      | openssl dgst -sha256 -hmac "$AUDITRAIL_SECRET" -hex | awk '{print $2}')
curl -sS "https://ats.ucentric.id/v1/events?action=smoke.test&limit=5" \
  -H "Authorization: Bearer $AUDITRAIL_API_KEY" \
  -H "X-Timestamp: $TS" -H "X-Nonce: $NONCE" \
  -H "X-Signature: hmac-sha256=$SIG"               # => { "count": 1, "items": [...] }
```

Kalau ketiganya sukses, integrasi Anda sudah benar. Siap deploy.

---

## Lampiran — Kesalahan umum

| Gejala                              | Akar masalah                                                      |
| ----------------------------------- | ----------------------------------------------------------------- |
| Selalu `401 invalid signature`      | JSON di‑serialize ulang setelah signing. Tandatangani **byte yang dikirim**. |
| Jalan di lokal, gagal di server     | Clock drift — NTP‑sync server.                                    |
| `409` acak di produksi              | Cache nonce / singleton menghasilkan UUID sama di beberapa thread.|
| Event sepertinya hilang             | Cek kategori — `log` / `metric` / `behavior` hidup di ClickHouse dengan TTL 14 hari; `audit` / `error` di PostgreSQL jangka panjang. |
| Lonjakan 5xx setelah traffic tinggi | Naikkan env var `RATE_LIMIT_*` di Coolify, atau pakai bulk ingest.|
| Integrasi browser membocorkan secret| Proxy lewat backend sendiri, atau terbitkan aplikasi public-ingest dengan limit ketat. |

---

Ada pertanyaan? Kirim **full request headers + body** (dengan `api_key`
di‑mask — **signature jangan diubah** supaya bisa didebug) ke pemilik
platform. Informasi itu cukup untuk mendiagnosis hampir semua kasus.
