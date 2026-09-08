# Security model

Senkaimon decides who reaches what. Caddy asks it before every proxied request and forwards the answer, so a backend behind the edge trusts whatever Senkaimon says about the caller.

This page states what that trust rests on: how each credential is stored, how long each one lives, and where the guarantee stops.

## Where the guarantee stops

Senkaimon is an edge control. It does not protect a backend from anything already inside the network.

- A LAN client can send a forged `Senkaimon-User` straight at a backend. Nothing behind the edge checks a second time, so an application that reads the identity headers must not be reachable except through the edge.
- A WebSocket is checked once, at the upgrade. Revoking the session does not close a socket already open.
- Senkaimon never sees a request body, a response, or a byte of proxied traffic. It reads three forwarded headers and answers with a status code.
- Policy decides on the host only. `X-Forwarded-Uri` reaches the decision but plays no part in it, so a policy cannot gate one path of a host and not another.

## Passwords

Argon2id, from `golang.org/x/crypto/argon2`, with a 16 byte random salt and a 32 byte output.

| Parameter | Default | Config key |
|---|---|---|
| Memory | 65536 KiB, so 64 MiB | `argon2.memory_kib` |
| Iterations | 3 | `argon2.time` |
| Parallelism | 4 | `argon2.threads` |

The parameters used for a hash are stored beside it. Verification reads them from the record rather than from the config, which is what lets you raise the config without locking anyone out: the next successful login for that user rehashes at the new cost. Comparison is `subtle.ConstantTimeCompare`.

A password is at least 12 characters. A username is 1 to 32 characters of `a-z`, `0-9`, dot, underscore, or hyphen.

Changing your own password requires the current one, so a stolen cookie alone cannot take the account over.

## Sessions

A session token is 32 random bytes, base64url encoded, handed out once in a cookie. What `sessions.json` holds is the SHA-256 of that value, so the file cannot be replayed out of a backup.

| Property | Value |
|---|---|
| Cookie flags | `Secure`, `HttpOnly`, `SameSite=Lax` |
| Cookie domain | `identity.cookie_domain`, so one login covers every subdomain |
| Idle expiry | `session.idle_ttl`, default 24h, measured from last use |
| Absolute expiry | `session.absolute_ttl`, default 720h, measured from issue |

Both expiries are enforced when a session is resolved and again on the background sweep, so an expired record cannot be used even before the sweep reaches it. Last-seen timestamps reach disk every `session.flush_interval` rather than on every request.

A half-finished login, meaning the state between a correct password and a correct second factor, is held in memory only and never written to disk. It carries its own cookie and expires after `session.pending_ttl`, default 5 minutes. Restarting the service drops every one of them.

## Second factor

TOTP as specified in RFC 4226 and RFC 6238: HMAC-SHA-1 over a 20 byte secret, 6 digits, a 30 second period, and one step of tolerance either side.

A consumed step is recorded on the user and every validation refuses a step at or below it. A code cannot be replayed inside its own window, which is what an attacker watching a shoulder or a proxy would otherwise get.

Recovery codes are ten values of 16 random bytes each, so 128 bits, base32 encoded. They are shown once at the end of enrolment and stored as SHA-256. Using one deletes it.

## Machine tokens

A token reads `senkaimon_<id>_<secret>`, where the id is 5 random bytes in lowercase base32 and the secret is 32 random bytes, base64url encoded.

Only the SHA-256 of the whole string is stored, and the comparison is constant time. The full value is shown once, at mint.

A token id can never collide with a username, because both are allocated out of the same namespace and each creation path checks the other. That matters because a policy names subjects as plain strings.

A token is never an admin. The admin check requires a principal of kind `user` carrying the admin flag, so a token presented to a management endpoint gets a 403 no matter which policies name it. Revoking a token also removes it from every policy that named it.

An expiry is optional. A token with none is valid until revoked.

## Rate limiting

Failures are counted in windows keyed three ways.

| Key | Counts |
|---|---|
| `user:<username>` | failed passwords for that account |
| `ip:<address>` | failed passwords from that client address |
| `totp:<username>` | failed second factors for that account |

A key locks once it reaches `ratelimit.max_failures`, default 5, inside `ratelimit.window`, default 15 minutes. The lockout lasts `ratelimit.lockout`, default 15 minutes. A successful password clears the username and address keys; the second factor counter is separate and survives discarding the login.

Locking per username means a known account can be locked out deliberately by failing five times. That is the accepted trade against unlimited guessing at a publicly reachable form.

The client address comes from `X-Forwarded-For`. Do not set `trusted_proxies` on the edge Caddy, because its default of trusting nobody is what makes that header the real client. When the header is absent Senkaimon logs a warning and per-address limiting is inactive for that request.

## Browser-facing endpoints

Every state-changing endpoint requires an `Origin` header whose scheme and host match `identity.idp_url` exactly. A missing `Origin` is a 403 rather than a pass, so a form posted from another site cannot reach them.

A login may carry an `rd` target to return to. A target is accepted only when all of the following hold:

1. It parses, uses `https`, and carries no userinfo component.
2. Its host sits inside `identity.cookie_domain`.
3. Its host is the IDP itself, or matches a host glob named in some policy.

The second condition is the load-bearing one. A `default` policy of `allow *` matches every host on the internet, so deriving the allowlist from policy globs alone would turn the login page into an open redirect.

## On disk

`~/.config/senkaimon/` is created at mode `0700` and every file inside it at `0600`. The path is hardcoded, with no flag and no XDG lookup.

| File | Holds |
|---|---|
| `config.yaml` | the settings above |
| `users.json` | accounts, password hashes, TOTP secrets, hashed recovery codes |
| `tokens.json` | token ids, owners, and hashes |
| `policies.json` | policies and their subjects |
| `sessions.json` | hashed session tokens |
| `audit.log` | one JSON object per line, appended and fsynced per event |

`users.json` holds TOTP secrets in plaintext, because a TOTP secret has to be replayable to verify a code. Anyone who can read that file can mint valid codes, which is the reason for the `0700` directory.

## Audit

Fifteen event types are recorded, each as one JSON line carrying a timestamp, the event name, and whichever of subject, address, host, and detail apply.

`login.success`, `login.failure`, `login.locked`, `totp.failure`, `totp.enrolled`, `recovery.used`, `session.revoked`, `token.minted`, `token.revoked`, `token.denied`, `user.created`, `user.updated`, `user.deleted`, `policy.updated`, `access.denied`.

Every write is followed by an fsync, so an event that reached the API reached the disk. The log has no rotation and no size cap, and the Audit view scans the whole file to show the tail.

## References

- How a policy reaches a verdict: [policies.md](policies.md)
- The edge configuration and its version floor: [caddy.md](caddy.md)
