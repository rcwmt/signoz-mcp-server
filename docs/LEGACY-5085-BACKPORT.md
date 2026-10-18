# Legacy SigNoz MCP compatibility backport

Tracks https://github.com/rcwmt/sendpoly-temporal/issues/5085.

Root explicitly admitted source `94f886101be4d4b833ef67d80c7ceb947373b8b7`
as the baseline. It is **not** proven byte-equivalent to the running
`cebff089341a54818d79489ff7d0fd4218cbb4e5+dirty` binary. The original
container, image, executable, checkout and customization receipts stay preserved.

The existing `signoz_execute_builder_query` accepts the same `query` object.
SQL now uses its own `name/query/disabled/legend` spec instead of losing SQL and
acquiring builder-only fields. Builder `groupBy` survives serialization. Explicit
`scalar`, `time_series`, `raw` and trace modes are preserved when compatible with
the signal, rather than rewritten according to query order. Unsupported query
types, fields, signals and modes fail closed before HTTP. Omitted modes retain
legacy single-signal defaults (metrics time series; logs/traces raw), with the
existing 60-second default step for time-series queries. Mixed signals must
specify a common compatible mode explicitly. This is not general PromQL/formula
or arbitrary new backend-schema support.

`signoz_get_dashboard` still requires `uuid` and returns JSON in the same MCP
text/error envelope. Only its GET destination changes to
`/api/v2/dashboards/{escaped-uuid}`. The complete v2 body is returned unchanged;
there is no invented v1 dashboard conversion, additional config request or
`uuid` to `id` migration. Other dashboard write/list tools are not modernized.

Startup, authentication, transports, all five local alert/channel additions,
Compose, module locks and dependencies are unchanged. No telemetry, scheduler or
upstream-wide workflow adoption is included.

## Offline consumer checks

`go test ./internal/handler/tools -run '^TestLegacyBackport' -count=1 -timeout=90s`
exercises the registered legacy MCP server, its real HTTP client, private Unix
socket backend, SQL/grouping/modes, upstream failures, unsupported-input
rejection, typed log/trace callers, v2 dashboard body and legacy tool schemas.
Query fixtures always return an error and never execute SQL. Dashboard success
is a static offline fixture, not evidence of a live dashboard or deployed fix.
No external backend or real credentials are used.

## Adoption is separate

This PR is source-only. Before runtime adoption, the original binding owner must
accept the behavioral changes and rollback plan, preserve the old artifact and
bindings, drain only this server's actual in-flight calls, and admit the exact
replacement artifact through an owner-safe path. No rebuild in place, global
MCP refresh, session restart or automatic deployment is authorized. Rollback
means restoring the preserved original artifact/binding after a safe drain;
perfect reconstruction of its historical dirty source is not a prerequisite to
an explicitly accepted future deployment.
