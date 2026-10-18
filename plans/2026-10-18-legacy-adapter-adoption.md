# Plan: Known-source replacement for the legacy query adapter

Status: Done
Issue: https://github.com/rcwmt/sendpoly-temporal/issues/5085
PR: Pending publication

## Context

The mounted `signoz-local` adapter rejected a ClickHouse SQL request after adding
builder-only fields. Its endpoint, immutable image and executable are now pinned;
its January 1 dirty source delta was not recovered from bounded retained history
or artifacts. This change prepares a replacement candidate, not an activation.

- Binding: loopback `http://127.0.0.1:8001/mcp`.
- Image: `sha256:37a806d35968ed575f9e8d70586fac112a26ba0b905a758b1d2310dc5ba03088`.
- Binary SHA256: `7ad17b7dbe4cd021e43605e479f24ce2b4e8288efe761cd8718a1e8291e63706`.
- Declared base: `cebff089341a54818d79489ff7d0fd4218cbb4e5`, `vcs.modified=true`;
  Go1.25.5 / mcp-go0.38.0. Exact dirty contents remain unknown.
- Local customized checkout: `94f886101be4d4b833ef67d80c7ceb947373b8b7`, committed
  after the mounted image was built; it cannot attest that image's complete source.
- Candidate base: upstream `2284b682cbe9b3696bf123bad54b2c079411a734`.

Upstream PR180 already fixed the typed SQL round trip. Current main replaced the
hand-copied query model with generated types; `handleExecuteBuilderQuery` forwards
the request map, applying only result bounds. Reimplementing the legacy mirror or
changing the SendPoly caller would add a second fix at the wrong boundary.

## Approach

Extend the existing handler test through registered MCP handlers, the official
SDK's in-memory transport, the actual HTTP client and a local HTTP receiver.
Compare complete SQL and grouped-signal requests at the receiver, with explicit
bounds so normalization need not alter them. The receiver always rejects with
HTTP400: verify the MCP result remains an error, including an unsupported SQL
field. No SQL engine, query result, live SigNoz request or ingestion is simulated.
This proves serialization and rejection propagation, not backend compatibility.
SigNoz remains responsible for validating unsupported query specs; no permissive
local success or duplicate query validator is introduced.

## Compatibility and custody map

| Surface | Legacy/local source | Candidate and required adoption action |
| --- | --- | --- |
| SQL/groupBy/signal | Builder-only mirror loses SQL/groupBy | Existing map-forwarding handler preserves type-specific fields; connected regression covers it. |
| Query tool | `signoz_execute_builder_query`, object `query` | Same name/query object; additional `searchContext` is advertised but not required. Registered schemas/results must be accepted by the original client before switching bindings. |
| MCP transport | mcp-go streamable HTTP `/mcp` | Official Go SDK, stateless HTTP `/mcp`, JSON responses, origin/body-size controls. In-memory proof does not attest mounted-client HTTP compatibility. |
| Backend authentication | Configured `SIGNOZ_URL` / `SIGNOZ_API_KEY` | Same names; synthetic API-key forwarding covered offline. Actual values were never read and must remain in the original secret owner. Header-supplied PAT/Bearer/OAuth modes are distinct; do not enable or change modes during adoption. |
| Alert/channel customizations | Local adds list/create/update/delete alert and list-notification-channel tools | All five names exist upstream; retain these capabilities rather than copying old implementations. Actual dirty-image additions beyond known source remain unknown. |
| Alert input | `alertRule` JSON string; update/delete `ruleId` | Direct rule object; canonical `id`, with `ruleId` alias retained for identifiers. Existing callers must migrate or receive an explicitly reviewed compatibility path; no silent payload translation is claimed. |
| Alert backend | `/api/v1/rules` | `/api/v2/rules`; backend support and rule semantics need original-owner admission, without a diagnostic mutation. |
| Channels/output | `/api/v1/channels`, raw results | `/api/v2/notification_channels`, configuration-free paginated summaries. Alert lists also return summaries; full definitions use `signoz_get_alert`. Do not silently discard callers relying on old outputs. |
| Exposure | `127.0.0.1:8001:8000` | Preserve loopback binding explicitly. Never adopt upstream Compose's default all-interface port mapping. |
| Network | Existing external `signoz-net` | Preserve it; do not replace it with upstream Compose's new bridge network. |
| Local docs/context | `CLAUDE.md`, `docs/INFRASTRUCTURE.md`, `docs/TOOLS.md`, `docs/WORKFLOWS.md` | Retain original checkout and docs; migrate useful context separately and correct stale examples before use. |
| Telemetry/docs refresh | Runtime effective configuration uninspected | Candidate adds runtime facilities. Do not enable exporters, analytics, remote documentation refresh or credentials as part of offline proof. Inspect/admit those separately before running a full candidate server. |

## Files to Modify

- `internal/handler/tools/query_builder_test.go`: connected consumer regression.
- This plan: compatibility, custody, candidate proof and activation limits.

## Key Decisions

### 2026-10-18 — Reuse the fixed upstream consumer

Preserve the mounted container/image/binary and original source checkout. Prepare
an immutable separate candidate from fresh upstream. Do not rebuild in place or
refresh/restart any client's MCP registration. Unknown dirty customizations are
explicit risk requiring owner acceptance; absence of retained history does not
justify claiming a clean source pin.

## Verification

The connected MCP-to-HTTP regression passed on Go1.27.1: all three scenarios
(SQL, grouped log/metric signals, unsupported SQL field) retained the complete
request and returned the real client's coded rejection from the offline receiver.
The receiver always returned HTTP400 and executed no SQL. `go.mod` and `go.sum`
SHA256 checks passed before and after the test. Dependencies were fetched once
using the configured official Go proxy and sum.golang.org validation, with no
checksum exceptions, into an isolated candidate cache. No module/lock change.
The earlier no-download attempt stopped at missing dependencies before tests ran.

Fleet heavy admission and a systemd scope enforced MemoryMax4GiB / CPUQuota200% /
swap0 / TasksMax128 and a finite900second fetch/compile/proof bound. The existing
complete suite was not repeated. The immutable candidate binary build and formal
Oracle result are separate publication receipts attached to the candidate PR;
this source plan does not claim their success ahead of observation.


## Outcome

Source candidate and compatibility map complete; no runtime change. Root/original binding owner must admit the known-source
replacement after compatibility assessment and review. Activation requires a safe
window with no interrupted active MCP call, preserved old artifact/rollback custody,
matching client transport/auth/tool schema and independently verified new binding
identity. A green offline test or a built binary is not a deployed fix; issue5085
remains open until the authorized actual consumer path is verified.
