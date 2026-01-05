# Infrastructure Reference

## Platform Overview

This is an **AI/LLM Inference Platform** with the following architecture:

```
                                    ┌─────────────────┐
                                    │  inference-     │
                                    │  console        │
                                    │  (UI)           │
                                    └────────┬────────┘
                                             │
┌─────────────┐    ┌─────────────────┐    ┌──▼──────────────┐
│ External    │───▶│ coolify-traefik │───▶│ inference-      │
│ Requests    │    │ (reverse proxy) │    │ gateway         │
└─────────────┘    └─────────────────┘    │ (main entry)    │
                                          └──┬──────────────┘
                          ┌─────────────────┬┴───────────────┬─────────────────┐
                          ▼                 ▼                ▼                 ▼
                   ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
                   │ inference-   │  │ inference-   │  │ inference-   │  │ cookie-      │
                   │ platform-api │  │ worker       │  │ admin        │  │ manager      │
                   └──────┬───────┘  └──────────────┘  └──────────────┘  └──────────────┘
                          │
         ┌────────────────┼────────────────┐
         ▼                ▼                ▼
  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
  │ tm-temporal- │ │ dify-api     │ │ langfuse-    │
  │ worker       │ │ (LLM apps)   │ │ web/worker   │
  │ (workflows)  │ │              │ │ (LLM obs)    │
  └──────────────┘ └──────────────┘ └──────────────┘
```

## Services

### Core Inference Services

| Service | Purpose | Traffic Level |
|---------|---------|---------------|
| **inference-gateway** | Main entry point, routes all inference requests | Very High (~18k) |
| **inference-platform-api** | Core API for inference operations | Medium (~147) |
| **inference-worker** | Processes inference jobs | Low (~1) |
| **inference-admin** | Admin operations and management | Medium (~657) |
| **inference-console** | Web UI for the platform | Medium (~627) |

### Supporting Services

| Service | Purpose | Traffic Level |
|---------|---------|---------------|
| **cookie-manager-service** | Session/cookie management | High (~1.2k) |
| **session-viewer** | View/debug user sessions | Low (~62) |
| **coolify-traefik** | Reverse proxy (Traefik via Coolify) | Low (~38) |

### Workflow & Processing

| Service | Purpose | Traffic Level |
|---------|---------|---------------|
| **tm-temporal-worker** | Temporal workflow worker | Low (~6) |
| **dataset-builder** | Builds/manages datasets | Very Low (~0.3) |

### LLM Tooling

| Service | Purpose | Traffic Level |
|---------|---------|---------------|
| **dify-api** | Dify LLM application platform | Very Low (~1) |
| **langfuse-web** | LangFuse web UI (LLM observability) | Very Low (~1.7) |
| **langfuse-worker** | LangFuse background worker | Very Low (~0.8) |

## Key Metrics

### Business Metrics
- `ticket_queue.tickets_inserted` - Tasks added to queue
- `ticket_queue.poll_iterations` - Queue polling cycles
- `ticket_queue.iteration_errors` - Queue processing errors
- `ticket_queue.queue_empty_iterations` - Empty queue polls

### HTTP Metrics (per service)
- Request latency (p50, p95, p99)
- Error rate (4xx, 5xx)
- Request count
- Request/response body sizes

### System Metrics
- CPU time and load average
- Process metrics (otelcol_process_cpu_seconds)

## Dashboards

| Name | UUID | Purpose |
|------|------|---------|
| Ticket_queue | `01998318-be16-748c-9f65-611d1bfc9c74` | Queue insertions by projectId |

## Service Relationships

### Request Flow (typical inference)
1. `coolify-traefik` → receives external request
2. `inference-gateway` → routes to appropriate service
3. `inference-platform-api` → handles API logic
4. `inference-worker` → processes inference job
5. May trigger `tm-temporal-worker` for complex workflows

### Session Flow
1. `inference-gateway` → `cookie-manager-service` (auth/session)
2. `session-viewer` → debugging/inspection

### LLM Observability Flow
1. Inference services → `langfuse-worker` (traces LLM calls)
2. `langfuse-web` → view traces and analytics

## Investigation Priorities

When investigating issues, check in this order:

1. **inference-gateway** - High traffic entry point, most likely source of user-facing issues
2. **inference-platform-api** - Core logic, business errors
3. **cookie-manager-service** - Auth/session issues
4. **tm-temporal-worker** - Workflow failures
5. **inference-worker** - Processing failures
