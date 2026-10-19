# Legacy runtime dashboard v2 follow-up

The three remaining dashboard list/create/update calls now use the supported v2 API. This extends the admitted legacy branch; it does not adopt modern upstream startup, dependencies, auth, transports or other tools.

Names `signoz_list_dashboards`, `signoz_create_dashboard`, `signoz_update_dashboard` and the update `uuid`/`dashboard` wrapper remain. List keeps its existing string `limit`/`offset` input and summary/pagination output, forwarding pagination to the server and advancing by the actual returned page size. Missing pagination data is an error, not an empty inventory.

Create/update now advertise writable v2 `name`, `schemaVersion: "v6"`, key/value `tags`, optional `image`, and the full Perses `spec`. Use a stable RFC1123 name. Automatic name generation is outside this minimal backport. V1 title/widgets/layout inputs fail closed; no lossy automatic conversion is attempted. The legacy dashboard authoring resources still describe v1 and are explicitly not referenced as valid v2 authoring instructions by these tools. Use the [official v2 guide](https://signoz.io/docs/dashboards/dashboards-v2-api/).

PUT replaces full state: first fetch the dashboard, copy only writable fields, preserve the COMPLETE spec including every panel, variable, layout, query, zero/null value and optional field, then apply intended edits. Read-only id/org/audit/source fields are not write payload fields. Missing panels/layouts/variables collections are rejected before a write to guard against accidental partial replacements. The API validates detailed plugin shapes, tags and names.

`dashboard_v2_test.go` drives real MCP JSON-RPC registrations, per-request auth and the real HTTP consumer against a private Unix backend. It checks paginated summaries and capped pages, v2 write methods/escaped IDs, complete panel payloads and error/unsupported-input handling. It does not mutate a live dashboard.

This changes only create/update input schemas, not tool names or the read/query tool catalog. Existing cached callers of these formerly broken write tools need the updated v2 schema before attempting writes; selective owner refresh is conditional on actual caller behavior, not a universal restart gate. No global refresh is authorized.

Use the #5085 exact build/package/cutover rails, including `docker update --restart unless-stopped <new full container ID>` immediately after Compose run, then verify that policy. Keep old rollback container3526/image37a until this follow-up is accepted live. Build/package/cutover and any bounded live write/cleanup proof require separate Root admission.
