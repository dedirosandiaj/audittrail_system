# Integrating with Go

Works in Go 1.21+. Uses only the standard library.

## Minimal client

```go
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
    Tags      map[string]any `json:"tags,omitempty"`
    Status    string         `json:"status,omitempty"`
}

func (c *Client) Send(e Event) error {
    if e.EventID == "" {
        e.EventID = uuid.NewString()
    }
    if e.Timestamp == "" {
        e.Timestamp = time.Now().UTC().Format(time.RFC3339)
    }
    if e.Severity == "" {
        e.Severity = "info"
    }
    body, _ := json.Marshal(e)

    ts    := strconv.FormatInt(time.Now().Unix(), 10)
    nonce := uuid.NewString()
    mac   := hmac.New(sha256.New, []byte(c.Secret))
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
    if h == nil {
        h = http.DefaultClient
    }
    resp, err := h.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        b, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("auditrail %d: %s", resp.StatusCode, b)
    }
    return nil
}
```

## Usage

```go
c := &auditrail.Client{
    APIKey:  os.Getenv("AUDITRAIL_API_KEY"),
    Secret:  os.Getenv("AUDITRAIL_SECRET"),
    BaseURL: "http://localhost:8080",
}

_ = c.Send(auditrail.Event{
    Category: "audit",
    Action:   "user.created",
    Actor:    map[string]any{"id": "admin_1", "type": "admin"},
    Resource: map[string]any{"id": "u_999",   "type": "user"},
    Payload:  map[string]any{"email": "x@y.z"},
})
```

## Fiber / Gin middleware

```go
func AuditMiddleware(c *fiber.Ctx) error {
    err := c.Next()
    go client.Send(auditrail.Event{
        Category: "behavior",
        Action:   "http.request",
        Payload: map[string]any{
            "method": c.Method(),
            "path":   c.Path(),
            "status": c.Response().StatusCode(),
        },
    })
    return err
}
```

Use a goroutine or a buffered channel to keep request latency flat.
