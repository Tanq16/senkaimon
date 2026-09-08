<div align="center">
  <img src="internal/server/static/icons/logo.svg" alt="Senkaimon Logo" width="180">
  <h1>Senkaimon</h1>

  <a href="https://github.com/Tanq16/senkaimon/actions/workflows/release.yaml"><img alt="Build Workflow" src="https://github.com/Tanq16/senkaimon/actions/workflows/release.yaml/badge.svg"></a>&nbsp;<a href="https://github.com/Tanq16/senkaimon/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/Tanq16/senkaimon"></a><br><br>
  <a href="#security">Security</a> &bull; <a href="#features">Features</a> &bull; <a href="#install">Install</a> &bull; <a href="#usage">Usage</a> &bull; <a href="#notes">Notes</a>
</div>

---

Senkaimon is a forward-auth identity service for a Caddy edge. It answers one question on every request, may this caller reach this host, and ships the web UI that manages the people, the machine tokens, and the policies behind that answer.

It exists so a home lab can be reachable from anywhere without every service growing its own login. It is not a proxy, it moves no bytes, and it is not an OIDC provider.

## Security

Senkaimon is the gate, and everything behind it trusts the answer it gives. The scope is worth reading before you run it.

- **It decides, it does not carry traffic.** Caddy asks `/verify` and acts on the status code. Senkaimon never sees a request body.
- **It guards the edge, not the LAN.** A client already inside the network can send a forged `Senkaimon-User` straight at a backend, because nothing behind the edge checks a second time.
- **Passwords are argon2id** at 64 MiB and 3 iterations, rehashed at the owner's next login whenever you raise the parameters.
- **A session is stored as the SHA-256 of its cookie.** The plaintext lives only in the browser, so `sessions.json` cannot be replayed out of a backup.
- **A machine token is never an admin.** It reaches a host only by being named as a subject in a policy, the same as a person.

The full model, with every lifetime, limit, and constraint, is in [docs/security.md](docs/security.md).

## Features

- **Two kinds of caller, one identity model.** A person signs in with a password and a TOTP code, a machine presents a bearer token, and both resolve to a subject that policy decides on.
- **Policies, not roles.** A policy names its subjects and a list of host globs, and a deny rule beats every allow. [How a policy decides](docs/policies.md).
- **TOTP built on the RFCs.** RFC 4226 and RFC 6238, HMAC-SHA-1, six digits, one step of tolerance either side, and a consumed step that can never be replayed.
- **Recovery codes.** Ten single-use codes of 128 bits each, shown once at the end of enrolment and stored hashed.
- **Rate limited on both factors.** Failures count per username and per client address, and the second factor keeps its own counter.
- **Append-only audit log.** Fifteen event types, one JSON object per line, written and fsynced as each event happens.
- **No database.** Six files under `~/.config/senkaimon/`, read into memory at start, so the verify path never touches disk.
- **One static binary.** No CGO, no runtime dependencies, and the whole web UI embedded.

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

Grab a binary from [releases](https://github.com/Tanq16/senkaimon/releases). Every push to `main` publishes `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`.

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

It writes `~/.config/senkaimon/` at mode `0700` with every file inside it at `0600`, creates the first admin, and gives that admin a `default` policy allowing every host. It refuses to run against an existing config unless given `--force`.

`--domain`, `--idp-url`, `--admin-user`, and `--admin-password` are prompted for when absent, so a bare `senkaimon setup` walks through them. `--admin-password -` reads the password from a pipe instead of shell history.

| Flag | Default | Notes |
|---|---|---|
| `--admin-user` | none | 1 to 32 characters of `a-z`, `0-9`, dot, underscore or hyphen |
| `--admin-password` | none | at least 12 characters, or `-` to read stdin |
| `--admin-totp` | `true` | the admin is written pending and enrols at first login |
| `--domain` | none | becomes the cookie domain as `.<domain>` |
| `--idp-url` | none | must be https and inside `--domain` |
| `--listen` | `127.0.0.1:4180` | |
| `--force` | `false` | overwrite an existing config |

### serve

```bash
senkaimon serve
```

Loads the config, reads every state file into memory, and binds. A missing config file is a fatal error rather than a fallback to defaults, since a server with no users cannot do anything useful.

Logs go to stdout as a styled console line on a terminal and as JSON everywhere else, so a collector under systemd gets structured records without a flag.

### Configuration

`config.yaml` is written by `setup` and hand-editable afterwards. An omitted key falls back to the default below. `identity.idp_url` and `identity.cookie_domain` have no default, so they must be present.

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

A token is an identity like any other, so it reaches a host by being named as a subject in a policy.

## Notes

- **The edge is Caddy 2.11.2 or newer.** Versions 2.10.0 through 2.11.1 carry GHSA-7r4p-vjf4-gxv4, where a client could send its own `Senkaimon-User` and have it reach the backend. The working Caddyfile is in [docs/caddy.md](docs/caddy.md).
- **Deployment is a native binary under systemd.** `/opt/senkaimon/versions/<version>/senkaimon` with `/opt/senkaimon/current` symlinked at it. `ProtectHome=yes` must not be set, because the config directory lives under the home directory, and `ReadWritePaths=` covers it instead.
- **A WebSocket is checked once, at the upgrade.** The tunnel that follows is never re-verified, so revoking a session does not close an open socket. Restart the service behind it when that matters.
- **The audit log is unbounded.** No rotation and no size cap in v1, and reading the Audit view scans the whole file.
- **Locking is per username, so a known account can be locked out deliberately** by failing five times. That is the accepted trade against unlimited guessing at a publicly reachable form.
- **Deleting a user cascades.** Their tokens, sessions, half-finished logins, and every mention of them as a policy subject go with them.
