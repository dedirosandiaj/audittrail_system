# Integrating with PHP

Works in PHP 8.0+. Uses only built-in functions (no Composer required).

## Minimal client

```php
<?php

final class Auditrail
{
    public function __construct(
        private string $apiKey,
        private string $secret,
        private string $baseUrl = 'http://localhost:8080',
    ) {}

    public function send(array $event): array
    {
        $event['event_id']  ??= $this->uuid();
        $event['timestamp'] ??= gmdate('Y-m-d\TH:i:s\Z');
        $event['severity']  ??= 'info';

        $body  = json_encode($event, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
        $ts    = (string) time();
        $nonce = $this->uuid();
        $sig   = hash_hmac('sha256', "$ts\n$nonce\n$body", $this->secret);

        $ch = curl_init("{$this->baseUrl}/v1/events");
        curl_setopt_array($ch, [
            CURLOPT_POST           => true,
            CURLOPT_POSTFIELDS     => $body,
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_HTTPHEADER     => [
                "Authorization: Bearer {$this->apiKey}",
                "X-Timestamp: $ts",
                "X-Nonce: $nonce",
                "X-Signature: hmac-sha256=$sig",
                "Content-Type: application/json",
            ],
            CURLOPT_TIMEOUT => 10,
        ]);
        $resp = curl_exec($ch);
        $code = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        curl_close($ch);

        if ($code >= 400) {
            throw new RuntimeException("auditrail $code: $resp");
        }
        return json_decode($resp, true);
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

## Laravel usage

```php
// bootstrap/providers or a facade wrapper
$audit = new Auditrail(env('AUDITRAIL_API_KEY'), env('AUDITRAIL_SECRET'),
                       env('AUDITRAIL_URL'));

$audit->send([
    'category' => 'audit',
    'action'   => 'invoice.created',
    'actor'    => ['id' => auth()->id(), 'type' => 'user'],
    'resource' => ['id' => $invoice->id, 'type' => 'invoice'],
    'payload'  => ['total' => $invoice->total],
]);
```

## Laravel error handler

```php
// app/Exceptions/Handler.php
public function report(Throwable $e)
{
    app(Auditrail::class)->send([
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

For production, push `send()` into a queued job (`dispatch()`) to avoid blocking
the HTTP response.
