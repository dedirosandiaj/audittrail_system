# @auditrail/sdk (JS)

Minimal Node.js 18+ SDK. No runtime dependencies.

```js
import { Auditrail } from "./index.js";

const client = new Auditrail({
  apiKey:  process.env.AUDITRAIL_API_KEY,
  secret:  process.env.AUDITRAIL_SECRET,
  baseUrl: "http://localhost:8080",
});

await client.send({
  category: "audit",
  action:   "user.login",
  actor:    { id: "u_1", type: "user" },
  resource: { id: "u_1", type: "user" },
});

await client.sendBulk([
  { category: "log", action: "app.debug",   payload: { msg: "a" } },
  { category: "log", action: "app.warning", severity: "warn", payload: { msg: "b" } },
]);
```

See `../../docs/integrations/javascript.md` for error capture recipes, browser
usage warning, and batching patterns.
