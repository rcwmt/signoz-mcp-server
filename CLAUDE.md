# SigNoz Observability Workspace

This workspace queries the AI/LLM Inference Platform via the SigNoz MCP server.

@docs/INFRASTRUCTURE.md
@docs/TOOLS.md
@docs/WORKFLOWS.md

## Quick Reference

### Key Services (by priority)
1. **inference-gateway** - Main entry point (highest traffic)
2. **inference-platform-api** - Core API logic
3. **cookie-manager-service** - Auth/sessions
4. **inference-worker** - Inference processing
5. **tm-temporal-worker** - Workflow orchestration

### Common Commands

**Error investigation:**
```
signoz_get_error_logs(timeRange='1h')
signoz_get_trace_error_analysis(timeRange='1h')
```

**Service health:**
```
signoz_get_service_top_operations(service='inference-gateway', timeRange='1h')
```

**Trace analysis:**
```
signoz_search_traces_by_service(service='<name>', error=true, limit=10)
signoz_get_trace_span_hierarchy(traceId='<id>')
```

**Queue status:**
```
signoz_search_metric_by_text(searchText='ticket_queue')
signoz_get_dashboard(uuid='01998318-be16-748c-9f65-611d1bfc9c74')
```

## Output Guidelines

1. **Summarize by service** when multiple are affected
2. **Show severity:** CRITICAL > HIGH > MEDIUM > LOW
3. **Include relationships:** service A → B → C
4. **Use tables** for comparisons
5. **Provide actionable insights**, not just data dumps

### Severity Classification
- **CRITICAL:** inference-gateway down (user-facing)
- **HIGH:** inference-platform-api, cookie-manager issues
- **MEDIUM:** Workers, background services
- **LOW:** Admin, viewer services

## Time Ranges
- `'1h'` - Recent issues (default)
- `'6h'` - Pattern analysis
- `'24h'` - Daily trends
- `'7d'` - Weekly trends
