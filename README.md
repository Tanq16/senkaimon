<div align="center">
  <img src="internal/server/static/icons/logo.svg" alt="Senkaimon Logo" width="180">
  <h1>Senkaimon</h1>

  <a href="https://github.com/Tanq16/senkaimon/actions/workflows/release.yaml"><img alt="Build Workflow" src="https://github.com/Tanq16/senkaimon/actions/workflows/release.yaml/badge.svg"></a>&nbsp;<a href="https://github.com/Tanq16/senkaimon/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/Tanq16/senkaimon"></a><br><br>
  <a href="#features">Features</a> &bull; <a href="#install">Install</a> &bull; <a href="#usage">Usage</a> &bull; <a href="#notes">Notes</a>
</div>

---

Senkaimon is a forward-auth identity service for a Caddy edge. It answers one question on every request, may this caller reach this host, and ships the web UI that manages the people, the machine tokens, and the policies behind that answer.

It exists so a home lab can be reachable from anywhere without every service growing its own login. It is not a proxy, it moves no bytes, and it is not an OIDC provider.

## Features

- **Two kinds of caller, one identity model.** A person authenticates with a password and a TOTP code, a machine with a bearer token, and both resolve to the same principal that policy decides on.
- **Policies, not roles.** A named policy lists subjects and host globs. A subject in no policy is denied, and a deny rule cannot be undone by an allow anywhere.
- **TOTP built on the RFCs.** RFC 4226 and RFC 6238, HMAC-SHA-1, six digits, a one-step window either side, and a consumed step that can never be replayed.
- **Recovery codes.** Ten single-use codes at 128 bits each, shown once at the end of enrolment and stored hashed.
- **Rate limited on both factors.** Failures are counted per username and per client address, and the second factor carries its own counter that survives discarding the login.
- **Append-only audit log.** Every login, denial, mint, and revocation as one JSON object per line.
- **No database.** Five files under `~/.config/senkaimon/`, loaded into memory at start, so the serving path never touches disk.
- **One static binary.** No CGO, no runtime dependencies, and the whole frontend embedded.

## Screenshots

<details>
<summary>Click to expand</summary>

### Sign in

| Enrolment |
| :---: |
| <img src=".github/assets/login.png" alt="TOTP enrolment" width="100%" /> |

### Management

| Users |
| :---: |
| <img src=".github/assets/users.png" alt="Users" width="100%" /> |

| Policies | Audit |
| :---: | :---: |
| <img src=".github/assets/policies.png" alt="Policies" width="100%" /> | <img src=".github/assets/audit.png" alt="Audit" width="100%" /> |

</details>

## Install

Grab a binary from [releases](https://github.com/Tanq16/senkaimon/releases). Every push to `main` publishes `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`. The edge runs `linux/arm64`.

```bash
curl -sfLo senkaimon https://github.com/Tanq16/senkaimon/releases/latest/download/senkaimon-linux-arm64
chmod +x senkaimon
```

Building from source needs Go 1.27 or newer and `curl`, which the asset target uses to fetch the pinned frontend files.

```bash
git clone https://github.com/Tanq16/senkaimon.git
cd senkaimon
make build
```

Nothing under `internal/server/static/css`, `js`, or `fonts` is committed. `make build` downloads it first, so a fresh clone compiles.

## Usage

Two commands. `--debug` is the only flag the whole tree honors, and it swaps the styled output for zerolog with the full error chain.

### setup

```bash
senkaimon setup --domain etherios.work --idp-url https://idp.etherios.work \
                --admin-user tanq --admin-password -
```

It writes `~/.config/senkaimon/` at mode `0700` with every file inside it at `0600`, creates the first admin, and gives that admin a `default` policy allowing every host. Any flag left out is prompted for, and `--admin-password -` reads the password from a pipe instead of shell history. It refuses to run against an existing config unless given `--force`.

| Flag | Default | Notes |
|---|---|---|
| `--admin-user` | none | prompts when absent |
| `--admin-password` | none | prompts when absent, or `-` to read stdin |
| `--admin-totp` | `true` | the admin is written pending and enrols at first login |
| `--domain` | none | becomes the cookie domain as `.<domain>` |
| `--idp-url` | none | must be https and inside `--domain` |
| `--listen` | `127.0.0.1:4180` | |
| `--force` | `false` | overwrite an existing config |

### serve

```bash
senkaimon serve
```

Loads the config, loads every state file into memory, and binds. A missing config is a fatal error rather than a fallback to defaults, since a server with no users cannot do anything useful.

### Configuration

`config.yaml` is written by `setup` and hand-editable afterwards. An omitted key falls back to the default below, and `identity.idp_url` and `identity.cookie_domain` have no default so they must be present.

| Key | Default | Description |
|---|---|---|
| `server.listen` | `127.0.0.1:4180` | Bind address, loopback only |
| `identity.issuer` | `Senkaimon` | The TOTP issuer label authenticator apps show |
| `identity.idp_url` | none | Absolute https URL the login page is served from |
| `identity.cookie_domain` | none | Parent domain, so one login covers every subdomain |
| `session.idle_ttl` | `24h` | Time since last use before a session expires |
| `session.absolute_ttl` | `720h` | Total session lifetime regardless of use |
| `session.pending_ttl` | `5m` | How long a half-finished login survives |
| `session.flush_interval` | `60s` | How often last-seen timestamps reach disk |
| `argon2.memory_kib` | `65536` | Argon2id memory cost |
| `argon2.time` | `3` | Argon2id iterations |
| `argon2.threads` | `4` | Argon2id parallelism |
| `ratelimit.max_failures` | `5` | Failures before a lockout |
| `ratelimit.window` | `15m` | Window the failures are counted in |
| `ratelimit.lockout` | `15m` | How long a lockout lasts |

Argon2 parameters are stored per user beside the hash, so raising them here rehashes each password at that user's next login rather than locking anyone out.

### Machine tokens

Mint one in the Tokens view. The full string is shown once and only its SHA-256 is stored.

```bash
curl -H "Authorization: Bearer senkaimon_k7m2q9xb_..." https://kairo.etherios.work/api/status
```

A token is an identity like any other, so it reaches a host by being named as a subject in a policy. A token is never an admin.

## Notes

- **The edge is Caddy 2.11.2 or newer.** Senkaimon answers `/verify` on loopback for `forward_auth`, and nothing routes that path publicly. Versions 2.10.0 through 2.11.1 carry GHSA-7r4p-vjf4-gxv4, where a client could send its own `Senkaimon-User` and have it reach the backend. The working Caddyfile is in [docs/caddy.md](docs/caddy.md).
- **Deployment is a native binary under systemd.** `/opt/senkaimon/versions/<version>/senkaimon` with `/opt/senkaimon/current` symlinked at it. `ProtectHome=yes` must not be set, because the config directory lives under the home directory, and `ReadWritePaths=` covers it instead.
- **A WebSocket is checked once, at the upgrade.** The tunnel that follows is never re-verified, so revoking a session does not close an open socket. Restart the service behind it when that matters.
- **The audit log is unbounded.** No rotation and no size cap in v1.
- **A redirect target must sit inside the cookie domain.** A session cookie set on `.etherios.work` could never reach another domain anyway, and the constraint is what stops a policy of `allow *` turning the login page into an open redirect.
- **Locking is per username, so a known account can be locked out deliberately** by failing five times. That is the accepted trade against unlimited guessing at a publicly reachable form.
- **The home side of the tunnel enforces nothing.** A LAN client can send a forged `Senkaimon-User` straight to the home Caddy. No application in this lab reads that header, and any application that starts to must sit behind the edge, where `copy_headers` deletes the client's version before setting its own.

