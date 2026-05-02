// Auditrail JS SDK (Node.js 18+)
// -----------------------------------------------------------------------------
// Minimal, dependency-free client that signs events with API key + HMAC-SHA256.
// Public API:
//   const client = new Auditrail({ apiKey, secret, baseUrl });
//   await client.send(event);                 // single
//   await client.sendBulk([event, event...]);  // up to 500 per request

import crypto from "node:crypto";
import { randomUUID } from "node:crypto";

export class Auditrail {
  /**
   * @param {{ apiKey: string, secret: string, baseUrl?: string, fetch?: typeof fetch }} opts
   */
  constructor(opts) {
    if (!opts?.apiKey) throw new Error("auditrail: apiKey is required");
    if (!opts?.secret) throw new Error("auditrail: secret is required");
    this.apiKey  = opts.apiKey;
    this.secret  = opts.secret;
    this.baseUrl = (opts.baseUrl ?? "http://localhost:8080").replace(/\/$/, "");
    this.fetch   = opts.fetch ?? globalThis.fetch;
  }

  /** Sign an arbitrary JSON body and return headers to include. */
  sign(body) {
    const ts    = Math.floor(Date.now() / 1000).toString();
    const nonce = randomUUID();
    const sig   = crypto
      .createHmac("sha256", this.secret)
      .update(`${ts}\n${nonce}\n${body}`)
      .digest("hex");
    return {
      "Authorization": `Bearer ${this.apiKey}`,
      "X-Timestamp":   ts,
      "X-Nonce":       nonce,
      "X-Signature":   `hmac-sha256=${sig}`,
      "Content-Type":  "application/json",
    };
  }

  async send(event) {
    const filled = this._fill(event);
    return this._post("/v1/events", JSON.stringify(filled));
  }

  async sendBulk(events) {
    if (!Array.isArray(events) || events.length === 0) {
      throw new Error("auditrail: events must be a non-empty array");
    }
    if (events.length > 500) {
      throw new Error("auditrail: bulk size exceeds 500");
    }
    const body = JSON.stringify({ events: events.map((e) => this._fill(e)) });
    return this._post("/v1/events/bulk", body);
  }

  _fill(event) {
    return {
      event_id:  event.event_id  ?? randomUUID(),
      timestamp: event.timestamp ?? new Date().toISOString(),
      severity:  event.severity  ?? "info",
      ...event,
    };
  }

  async _post(path, body) {
    const res = await this.fetch(`${this.baseUrl}${path}`, {
      method:  "POST",
      headers: this.sign(body),
      body,
    });
    const text = await res.text();
    if (!res.ok) {
      throw new Error(`auditrail ${res.status}: ${text}`);
    }
    return text ? JSON.parse(text) : {};
  }
}

export default Auditrail;
