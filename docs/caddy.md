# The Caddy edge

Caddy is the only thing that talks to Senkaimon's `/verify` endpoint. Before proxying a request to a gated host, Caddy asks Senkaimon about the caller and acts on the status code it gets back.

Senkaimon listens on loopback and is never routed publicly, apart from the IDP host that serves the login page.

## Version floor

**Caddy 2.11.2 or newer.** Versions 2.10.0 through 2.11.1 carry GHSA-7r4p-vjf4-gxv4, where `copy_headers` did not unconditionally delete the destination header before writing it. On those versions a client could send its own `Senkaimon-User` and have it survive to the backend, which defeats the whole arrangement.

## The Caddyfile

```caddyfile
# The IDP itself. No forward_auth here, or logging in is impossible.
idp.etherios.work {
	reverse_proxy 127.0.0.1:4180
}

(gated) {
	forward_auth 127.0.0.1:4180 {
		uri /verify
		copy_headers Senkaimon-User Senkaimon-Kind Senkaimon-Token-Id
	}
	request_header -Authorization
}

kairo.etherios.work {
	import gated
	reverse_proxy 192.168.0.11:8081
}
```

Every gated host imports the one snippet, so adding a service is two lines and cannot drift from the others.

## Three things not to do

**Do not put `forward_auth` on the IDP host.** The login page has to be reachable without a session, and gating it makes signing in impossible.

**Do not add a manual strip for the three identity headers.** The route that `copy_headers` generates already deletes them before setting its own, and `request_header` runs afterwards, so a strip would delete the identity Senkaimon just wrote.

**Do not set `trusted_proxies` on the edge.** Its default of trusting nobody is what makes `X-Forwarded-For` the real client address, which Senkaimon's rate limiter keys on. Widening it lets a caller spoof the address that limits them.

`request_header -Authorization` is the one strip you do want. It stops a Senkaimon token reaching an application that has no business seeing it.

## What Senkaimon reads

`forward_auth` sets the `X-Forwarded-*` family itself, so these arrive without extra configuration.

| Header | Used for |
|---|---|
| `X-Forwarded-Host` | the host the policy decides on; a request without it is a 400 |
| `X-Forwarded-Uri` | the return address for the login redirect, and nothing else |
| `X-Forwarded-Proto` | the scheme of that return address, assumed `https` when absent |
| `X-Forwarded-For` | the client address the rate limiter counts against |
| `Authorization` | a `Bearer` value is resolved as a machine token instead of a session |
| `Accept` | a request that does not accept `text/html` gets a 401 rather than a redirect |

That last row is what keeps an API client from being handed a login page. A browser follows the redirect; `curl` gets a status code.

## What Senkaimon answers

| Response | What Caddy does |
|---|---|
| `204` | copies the identity headers onto the request and continues upstream |
| `401` | relays it, with `WWW-Authenticate: Bearer` when a bearer token was presented |
| `403` | relays it, which is what a valid credential failing policy gets |
| `302` | relays it, so a browser lands on the login page and returns afterwards |

## The identity headers

Set on a `204`, and the three that `copy_headers` carries upstream.

| Header | Value |
|---|---|
| `Senkaimon-User` | the owning person, so a machine token reports the human it belongs to rather than its own id |
| `Senkaimon-Kind` | `user` or `token` |
| `Senkaimon-Token-Id` | the token id, present only when the caller presented a bearer token |

An application behind the edge can read these to tell who it is serving. It must not be reachable except through the edge, because a LAN client can set the same headers directly.

## References

- What decides the verdict: [policies.md](policies.md)
- What the guarantee rests on and where it stops: [security.md](security.md)
