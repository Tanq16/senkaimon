# Policies

A policy names a set of subjects and a list of host rules. Senkaimon gathers the rules of every policy naming the caller, runs them in a fixed order, and answers allow or deny.

There are no roles and no inheritance. A subject is a username or a machine token id, written as a plain string, and the two share one namespace so a token id can never collide with a username.

## The decision

For a caller and a host, in this order:

1. Collect the rules of every policy whose subjects include the caller.
2. No rules at all, so the caller is in no policy. **Denied**, recorded as `no policy`.
3. Any `deny` rule whose glob matches the host. **Denied**, recorded as `explicit deny`.
4. No `allow` rules among what was collected. **Allowed**, recorded as `deny-only policy`.
5. Any `allow` rule whose glob matches the host. **Allowed**.
6. Nothing matched. **Denied**, recorded as `not permitted`.

Step 3 runs before step 5 and does not depend on where the rules sit, so a deny beats an allow whether it was written above it, below it, or in a different policy.

Step 4 is the one that surprises people. A subject carrying only `deny` rules reaches everything those rules do not name, because a list of exceptions with no grant is read as "everything but these". Give a subject at least one `allow` rule whenever you mean it to reach only certain hosts.

Denials appear in the audit log as `access.denied`, with the reason above in the `detail` field. An allow writes no event.

## Host globs

A rule's host is a glob matched against the forwarded host with Go's `path.Match`. The host is lowercased and any port is stripped first.

| Glob | Matches | Does not match |
|---|---|---|
| `kairo.etherios.work` | that host exactly | anything else |
| `*.etherios.work` | `kairo.etherios.work`, and also `a.b.etherios.work` | `etherios.work` |
| `*` | every host, including hosts you do not own | nothing |

A dot is an ordinary character to `path.Match`, so a single `*` spans as many labels as it needs to. `*.etherios.work` is not one level deep.

`setup` writes a `default` policy giving the first admin `allow *`. That is a deliberate starting point rather than a recommendation, and it is worth narrowing once real hosts exist.

## Paths are not considered

A policy decides on the host alone. Caddy forwards the request path and Senkaimon reads it, but only to build the return address for the login redirect. The path plays no part in the verdict.

Gating one path of a host and not another needs two hosts, or an application that does its own check on the identity headers.

## What a policy accepts

A policy is rejected on write unless all of the following hold:

- The name is not empty.
- It carries at least one rule.
- Every rule's effect is exactly `allow` or `deny`.
- Every rule's host glob is not empty.
- Every subject already exists as a user or as a token id.

A policy with no subjects is accepted and applies to nobody.

## When subjects disappear

Senkaimon removes a subject from every policy that named it rather than leaving a dangling string behind.

- Revoking a token drops its id from every policy.
- Deleting a user drops their username, and the ids of every token they owned, from every policy. Their sessions and half-finished logins go too.

A policy left with no subjects stays on disk. It applies to nobody until you name someone in it again.

## Where else the globs are read

The host globs across all policies also form the allowlist for the login page's `rd` return target, alongside the IDP's own host.

That allowlist is additionally constrained to `identity.cookie_domain`, which is what stops a `default` policy of `allow *` turning the login page into an open redirect.

## References

- Credential storage, lifetimes, and the redirect rules: [security.md](security.md)
- The edge configuration that asks for a verdict: [caddy.md](caddy.md)
