# Contrato: documentación del provider

## `Description` del esquema del provider (→ `docs/index.md`)

Texto actual (final):

```text
… Requests rejected with 429 (rate limit) or 503 (server busy) are retried honouring Retry-After for up to 10 minutes per request; each API token has its own rate-limit quota.
```

Texto nuevo (final):

```text
… Requests rejected with 429 (rate limit) or 503 (server busy) are retried honouring Retry-After for up to 10 minutes per request. Each API token has its own rate-limit quota; in addition, IPzilon >= 3.4.0 blocks API-token requests from a client address for up to a minute ("Too many invalid API tokens") after too many requests with non-existent tokens from that address, even when the token in use is valid. Those 429s are retried the same way.
```

El resto de la descripción no cambia. `docs/index.md` se regenera con `make generate`.

## `README.md` — sección *Compatibility*

Párrafo actual:

```markdown
Requests rejected by IPzilon with `429` (rate limit) or `503` (server busy) are
retried automatically, honouring `Retry-After`, for up to 10 minutes per
request. Each API token has its own rate-limit quota, so use one token per
pipeline if runs should not share it.
```

Se mantiene y se añade a continuación:

```markdown
Since IPzilon 3.4.0 there is also a per-client-address block: after too many
requests with an API token that **does not exist** (30 per minute by default,
`RATE_LIMIT_INVALID_API_TOKEN` in IPzilon), every request with an API token
from that address gets `429` `Too many invalid API tokens` for one window
(60 s by default), **even with a valid token**. Revoked or expired tokens do
not count. The provider waits it out like any other `429`, so a `plan` or
`apply` may pause for up to a minute but does not fail. Since provider v3.2.1,
with `TF_LOG=WARN` it also logs a warning naming the cause. If it happens, look
for another process behind the same public address (NAT, shared runner) using a
deleted or mistyped token, or check the provider's own token.
```
