# Authorization System

## Overview

This document describes the target authorization architecture for the Balda game server — replacing the current mixed-header scheme with a standard JWT Bearer token flow plus explicit HTTP-layer resource ownership checks.

---

## Current State and Problems

The server currently has three overlapping auth mechanisms:

| Mechanism | Header | Scope | Problem |
|-----------|--------|-------|---------|
| Static API key | `X-API-Key` | Server-wide, one value | Not per-user; leaking it grants access for everyone |
| Redis session | `X-API-Session` | Per-user, 5-min TTL | Requires constant ping or client gets 401 mid-game |
| DB `api_key` column | — | Per-user UUID in `users` | Returned to client but never verified |

Additional issues:

1. **Two tokens per request** — clients must attach both `X-API-Key` and `X-API-Session` on every authenticated call; non-standard and error-prone.
2. **No refresh token pattern** — session expiry means re-login; no graceful silent renewal.
3. **No HTTP-layer ownership checks** — whether a player belongs to a game is only verified deep inside the FSM (`internal/game/`), so unauthorized requests waste lobby and storage round-trips before being rejected.
4. **No roles** — all authenticated users are equal; no way to grant admin capabilities.

---

## Target Architecture

### Token Model

Two-token scheme, modelled on OAuth 2.0 best practices:

#### Access Token (JWT)

```
Header: { "alg": "HS256", "typ": "JWT" }
Claims: {
  "sub":  "<uid_string>",   // RFC 7519: string; strconv.FormatInt(uid, 10)
  "uid":  <uid_int64>,      // custom claim for direct Go use (avoids string parsing in hot path)
  "pid":  "<player_uuid>",  // player UUID (public game identifier)
  "role": "player",         // "player" | "admin"
  "jti":  "<uuid>",         // unique token ID (required for logout blacklisting)
  "iat":  <unix>,
  "exp":  <unix>             // iat + 3600 (1 hour)
}
Signature: HMAC-SHA256(header + "." + claims, JWT_SECRET)
```

- Stateless — validated by signature check, no Redis or DB lookup on every request.
- Short TTL (1 hour) limits blast radius of a stolen token.
- Transmitted in `Authorization: Bearer <token>` header.

#### Refresh Token

- Opaque random string: `hex(32 random bytes)` — 64-char hex string.
- Only an **HMAC-SHA256 hash** of it is stored in the DB (`refresh_tokens.token_hash`):
  `HMAC-SHA256(JWT_SECRET, rawToken)` — deterministic, enabling indexed lookup; 32 bytes of token entropy make brute-force infeasible without a slow hash.
- TTL: 30 days.
- **Rotated on each use**: presenting a refresh token issues a new pair and invalidates the old one.
- Revoked on logout and on suspicious replay detection.

### Authentication Flows

#### Signup

```
POST /signup
Body: { "firstname", "lastname", "email", "password" }

Server:
  1. Hash password with bcrypt (cost 12) in Go (move hashing out of Postgres)
  2. INSERT into users + player_state (transaction)
  3. Generate access_token JWT
  4. Generate refresh_token → bcrypt-hash → INSERT into refresh_tokens
  5. Generate Centrifugo connection + lobby subscription JWTs

Response 201:
{
  "player": { "uid", "firstname", "lastname", "pid", "exp" },
  "access_token":  "<jwt>",
  "refresh_token": "<opaque>",
  "token_type":    "Bearer",
  "expires_in":    3600,
  "centrifugo": {
    "connection_token":    "<jwt>",
    "subscription_token":  "<jwt>"
  }
}
```

#### Login

```
POST /auth
Body: { "email", "password" }

Server:
  1. Fetch user record by email
  2. bcrypt.CompareHashAndPassword(stored_hash, password)
  3. Generate access_token JWT
  4. Upsert refresh_token (invalidate previous if exists for this device)
  5. Generate Centrifugo tokens (+ active_game token if player is mid-game)

Response 200: same shape as signup
```

#### Token Refresh (silent renewal)

```
POST /auth/refresh
Body: { "refresh_token": "<opaque>" }

Server:
  1. Lookup refresh_tokens by token_hash
  2. Check revoked=false AND expires_at > now()
  3. Mark old row as revoked=true
  4. Generate new access_token + new refresh_token (rotation)
  5. INSERT new refresh_token row

Response 200:
{
  "access_token":  "<new jwt>",
  "refresh_token": "<new opaque>",
  "token_type":    "Bearer",
  "expires_in":    3600
}

Error 401: { "code": "token_expired" } or { "code": "token_revoked" }
```

Client-side strategy: attempt refresh when any request returns 401 with
`WWW-Authenticate: Bearer error="invalid_token"`, then retry original request once.

#### Logout

```
POST /auth/logout
Authorization: Bearer <access_token>
Body: { "refresh_token": "<opaque>" }   // optional; revokes this device's refresh token

Server:
  1. Validate access_token (must be valid)
  2. Mark refresh_token row revoked=true (if provided)
  3. Add jti to Redis blacklist with TTL = remaining access_token lifetime
     Key: "jwt:blacklist:<jti>", Value: "1", TTL: claims.exp - now()
     Without this step a logged-out access token remains valid for up to 1 hour.
     BearerAuth middleware must check this key after signature verification.

Response 204
```

### Authorization Model

#### Middleware Chain

```
Incoming request
  │
  ├─ BearerAuth middleware (ogen SecurityHandler)
  │     ├── Extract "Authorization: Bearer <token>" header
  │     ├── jwt.Parse(token, secret) → verify signature + exp
  │     ├── Inject Claims{UID, PlayerID, Role} into request context
  │     └── → 401 WWW-Authenticate: Bearer  if invalid/missing
  │
  └─ Handler
        ├── [game action endpoints only] GameOwnership check
        │     ├── lobby.IsParticipant(ctx, uid, gameID)
        │     └── → 403 Forbidden  if player is not in this game
        │
        ├── [admin endpoints only] RoleCheck middleware
        │     └── → 403 Forbidden  if role != "admin"
        │
        └── Business logic
```

#### Endpoint Access Matrix

| Endpoint | Public | Authenticated | Owner of game | Admin |
|----------|:------:|:-------------:|:-------------:|:-----:|
| `POST /signup` | ✓ | | | |
| `POST /auth` | ✓ | | | |
| `POST /auth/refresh` | ✓ | | | |
| `POST /auth/logout` | | ✓ | | |
| `GET /player/state/{uid}` | ✓ | | | |
| `POST /games` | | ✓ | | |
| `GET /games` | | ✓ | | |
| `POST /games/{id}/join` | | ✓ | | |
| `POST /games/{id}/move` | | | ✓ | |
| `POST /games/{id}/propose-end` | | | ✓ | |
| `POST /games/{id}/accept-end` | | | ✓ | |
| `POST /games/{id}/reject-end` | | | ✓ | |
| `POST /games/{id}/skip` | | | ✓ | |
| `DELETE /games/{id}` _(future)_ | | | | ✓ |

**Owner of game** = player whose `uid` (from JWT `sub` claim) is a current participant in the
lobby game identified by `{id}`. The check calls `lobby.IsParticipant` before handing off to the
service layer.

#### Roles

| Role | Granted by | Capabilities |
|------|-----------|--------------|
| `player` | Default on signup | Play games, manage own session |
| `admin` | Manual DB update (`UPDATE users SET role='admin' WHERE ...`) | Future: force-end games, view all active games, ban players |

Role is stored in `users.role TEXT NOT NULL DEFAULT 'player'` and embedded in the JWT at login/signup time. Changing a role requires re-login to take effect (no live token invalidation needed for now).

---

## Security Considerations

### Token Secrets
- `JWT_SECRET`: minimum 32 random bytes, loaded from env; never logged or returned in responses.
- Rotate by changing the env var and forcing re-login (all existing access tokens become invalid immediately; refresh tokens remain valid until a new access token is requested with the new secret, at which point verification fails and the client must re-login).

### Refresh Token Storage
- Only an **HMAC-SHA256 hash** (`HMAC-SHA256(JWT_SECRET, rawToken)`) of the opaque token is stored in the DB — a DB dump without the secret does not expose usable tokens.
- Unlike bcrypt, HMAC-SHA256 is deterministic, enabling direct indexed lookup (`WHERE token_hash = $1`). bcrypt is unsuitable here because its non-deterministic output (salted) makes SQL lookup impossible without a full-table scan.
- The raw token is sent to the client once and never stored server-side.

### Replay Attack Prevention
- Refresh token rotation: presenting a refresh token that was already rotated (i.e., `revoked=true`) signals a replay attack. The server should immediately revoke **all** refresh tokens for that user and force re-login.

### Rate Limiting
| Endpoint | Limit | Window |
|----------|-------|--------|
| `POST /auth` | 10 requests | 1 min / IP |
| `POST /auth/refresh` | 20 requests | 1 min / IP |
| `POST /signup` | 5 requests | 1 min / IP |

Implement as middleware using a sliding window counter in Redis (reuse existing Redis client).

### HTTPS
All tokens must be transmitted over TLS only. Configure the reverse proxy (nginx/caddy) to set
`Strict-Transport-Security: max-age=63072000` and redirect plain HTTP to HTTPS.

### Token Lifetime Summary
| Token | TTL | Storage |
|-------|-----|---------|
| Access token (JWT) | 1 hour | Client only (memory / secure storage) |
| Refresh token | 30 days | DB (`refresh_tokens`) — HMAC-SHA256 hashed |
| Centrifugo connection JWT | 24 hours | Client only |
| Centrifugo subscription JWT | 24 hours | Client only |

> **Note:** Centrifugo JWTs have a 24-hour TTL while access tokens expire in 1 hour. This means
> a revoked user can still send WebSocket events for up to 24 hours. Acceptable for the current
> scale; if tighter revocation is needed, reduce Centrifugo TTL to match the access token and
> re-issue Centrifugo tokens on each access token refresh.

---

## Implementation Guide

### Database Changes

**Migration 004** — add `role` to users, add `refresh_tokens` table, drop unused `api_key`:

```sql
-- 004_auth_tokens.up.sql

ALTER TABLE users
  ADD COLUMN role TEXT NOT NULL DEFAULT 'player',
  DROP COLUMN api_key;

CREATE TABLE refresh_tokens (
  token_id   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    BIGINT      NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
  token_hash TEXT        NOT NULL UNIQUE,   -- HMAC-SHA256(JWT_SECRET, rawToken)
  issued_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  revoked    BOOLEAN     NOT NULL DEFAULT false,
  user_agent TEXT,
  ip_addr    INET
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_expires ON refresh_tokens(expires_at) WHERE revoked = false;
```

```sql
-- 004_auth_tokens.down.sql

DROP TABLE refresh_tokens;
ALTER TABLE users
  DROP COLUMN role,
  ADD COLUMN api_key UUID DEFAULT gen_random_uuid();
```

### Storage Layer

New file: `internal/storage/token.go`

```go
// SaveRefreshToken stores an HMAC-SHA256-hashed refresh token.
func (s *Balda) SaveRefreshToken(ctx context.Context, uid int64, tokenHash string, expiresAt time.Time, ua, ip string) error

// GetRefreshToken fetches a non-revoked token row by hash.
func (s *Balda) GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshToken, error)

// RevokeRefreshToken marks a single token as revoked.
func (s *Balda) RevokeRefreshToken(ctx context.Context, tokenHash string) error

// RevokeAllUserTokens marks all tokens for a user as revoked (replay attack response).
func (s *Balda) RevokeAllUserTokens(ctx context.Context, uid int64) error
```

### JWT Package

New file: `internal/auth/jwt.go`

```go
type Claims struct {
    // Sub is the string representation of the internal user_id (int64).
    // RFC 7519 §4.1.2 defines "sub" as a string — store int64 separately.
    UID      int64     `json:"uid"`
    PlayerID uuid.UUID `json:"pid"`
    Role     string    `json:"role"`
    jwt.RegisteredClaims // RegisteredClaims.Subject (string) carries strconv.FormatInt(uid, 10)
}

func GenerateAccessToken(uid int64, pid uuid.UUID, role, secret string) (string, error)
func ParseAccessToken(tokenStr, secret string) (*Claims, error)
```

> **Note on `sub`:** RFC 7519 defines `sub` as a string. Set `RegisteredClaims.Subject = strconv.FormatInt(uid, 10)` and keep `uid int64` as a custom claim for direct use in Go. This avoids type-mismatch rejections by client-side JWT libraries that strictly validate the `sub` type.

Reuse `golang-jwt/jwt` (already a transitive dependency via `internal/centrifugo/token.go`).

Add to the same file or `internal/auth/token.go`:

```go
// HashRefreshToken returns HMAC-SHA256(jwtSecret, rawToken) as a hex string.
// Deterministic — safe for use as a unique DB index key.
func HashRefreshToken(secret, rawToken string) string {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(rawToken))
    return hex.EncodeToString(mac.Sum(nil))
}
```

### OpenAPI Spec Changes (`api/openapi/http-api.yaml`)

Replace the two existing security schemes:

```yaml
# Remove:
securitySchemes:
  APIKeyHeader:
    type: apiKey
    in: header
    name: X-API-Key
  APIKeyQueryParam:
    type: apiKey
    in: query
    name: api_key

# Add:
securitySchemes:
  BearerAuth:
    type: http
    scheme: bearer
    bearerFormat: JWT
```

Update all protected endpoints:
```yaml
# Before:
security:
  - APIKeyHeader: []
  - APIKeyQueryParam: []

# After:
security:
  - BearerAuth: []
```

Add new endpoint schemas for `POST /auth/refresh` and `POST /auth/logout`.

After editing the spec, run:
```bash
make code-gen
```

### Security Handler (`internal/server/restapi/handlers/handlers.go`)

Replace `HandleAPIKeyHeader` / `HandleAPIKeyQueryParam` with:

```go
func (h *Handlers) HandleBearerAuth(ctx context.Context, operationName ogen.OperationName, t ogen.BearerAuth) (context.Context, error) {
    claims, err := auth.ParseAccessToken(t.Token, h.jwtSecret)
    if err != nil {
        return ctx, &ogen.SecurityError{Err: err}
    }
    ctx = auth.WithClaims(ctx, claims)
    return ctx, nil
}
```

Remove the `xAPIToken` field and `--server.x_api_token` CLI flag from `cmd/server.go`.
Add `--auth.jwt_secret` flag (or `JWT_SECRET` env var).

### New Handlers

- `internal/server/restapi/handlers/refresh.go` — `POST /auth/refresh`
- `internal/server/restapi/handlers/logout.go` — `POST /auth/logout`

### Game Ownership Middleware

Add to `internal/lobby/lobby.go`:
```go
// IsParticipant returns true if the player identified by uid is currently in the given game.
func (l *Lobby) IsParticipant(gameID string, uid int64) bool
```

Call it at the top of game action handlers (move, propose-end, accept-end, reject-end, skip)
before delegating to the service layer. Return `403 Forbidden` on failure.

### Migration Path

| Step | Action | Breaking change |
|------|--------|-----------------|
| 1 | Add `docs/auth-system.md` | No |
| 2 | Run migration 004 (add `role`, add `refresh_tokens`, drop `api_key`) | `api_key` removed from DB |
| 3 | Implement `internal/auth/jwt.go` | No |
| 4 | Implement `internal/storage/token.go` | No |
| 5 | Update OpenAPI spec + `make code-gen` | No (additive) |
| 6 | Update `/auth` and `/signup` to return JWT pair | Yes — response shape changes |
| 7 | Add `POST /auth/refresh`, `POST /auth/logout` | No (new endpoints) |
| 8 | Replace session-based identity extraction with JWT claims in all handlers | Yes — `X-API-Session` no longer accepted |
| 9 | Add `lobby.IsParticipant` ownership check | No (stricter, but correct) |
| 10 | Remove `--server.x_api_token` flag, add `--auth.jwt_secret` | Config change |
| 11 | Deprecate `POST /session/ping` (return 410 Gone for one release, then remove) | Soft |

Steps 1–5 are non-breaking and can be merged first. Steps 6–8 are the breaking cutover — coordinate
with client teams.

---

## Glossary

| Term | Definition |
|------|-----------|
| Access token | Short-lived JWT proving identity; attached to every authenticated request |
| Refresh token | Long-lived opaque token used only to obtain a new access token |
| Token rotation | Practice of issuing a new refresh token on each refresh and invalidating the old one |
| `jti` | JWT ID — unique identifier per token; used for optional blacklisting |
| Bearer token | HTTP authentication scheme where the token is passed as `Authorization: Bearer <token>` |
| RBAC | Role-based access control — permissions granted based on user role |
| Owner of game | A player who is a current participant in a specific game instance in the lobby |
