# Caddy edge

Senkaimon answers `/verify` on loopback for Caddy's `forward_auth`. Caddy asks before every proxied request, and Senkaimon replies with a status code plus, when the caller is known, three identity headers the backend can read.

**Caddy 2.11.2 or newer is required.** Versions 2.10.0 through 2.11.1 carry GHSA-7r4p-vjf4-gxv4, where `copy_headers` did not unconditionally delete the destination header first, so a client could send its own `Senkaimon-User` and have it reach the backend.

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

## What each directive is doing

`request_header -Authorization` stops a Senkaimon token reaching an application that has no business seeing it.

Do not add a manual strip for the three identity headers. The route `copy_headers` generates already deletes them, and `request_header` runs afterwards, so a strip would delete the identity Senkaimon just set.

Do not set `trusted_proxies` on the edge Caddy. Its default of trusting nobody is what makes `X-Forwarded-For` the real client address, which the rate limiter keys on.

Nothing routes `/verify` publicly. It is reachable only as the loopback subrequest.

## What Senkaimon answers, and what Caddy does with it

| Response | Caddy |
|---|---|
| `204` | copies the identity headers onto the request and continues upstream |
| `401` | relays it to the client, with `WWW-Authenticate` when a bearer token was presented |
| `403` | relays it, which is what a valid credential failing policy gets |
| `302` | relays it, so a browser lands on the login page |

## Identity headers

These are set on a `204` and are what `copy_headers` carries upstream.

| Header | Value |
|---|---|
| `Senkaimon-User` | the owning person, so a machine token reports the human it belongs to rather than its own id |
| `Senkaimon-Kind` | `user` or `token` |
| `Senkaimon-Token-Id` | the token id, set only when the caller presented a bearer token |
