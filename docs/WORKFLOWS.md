# Common Workflows

## 1. Error Investigation

**User asks:** "Do any services have errors? Show me the most severe and how they're connected."

### Step 1: Get Error Overview
```
signoz_get_error_logs(timeRange='1h', limit=100)
signoz_list_alerts()
signoz_get_trace_error_analysis(timeRange='1h')
```

### Step 2: Identify Affected Services
Group errors by service. Priority order:
1. inference-gateway (user-facing)
2. inference-platform-api (core logic)
3. cookie-manager-service (auth)
4. tm-temporal-worker (workflows)

### Step 3: Get Service Context
For each affected service:
```
signoz_get_service_top_operations(service='<name>', timeRange='1h')
signoz_search_traces_by_service(service='<name>', error=true, limit=10)
```

### Step 4: Trace Connections
Pick error traces and analyze relationships:
```
signoz_get_trace_details(traceId='<id>')
signoz_get_trace_span_hierarchy(traceId='<id>')
```

### Step 5: Output Format
```
## Error Summary by Service

### inference-gateway (CRITICAL)
- **Error count:** 47 in last hour
- **Most common:** Connection timeout to inference-worker
- **Impact:** User requests failing

### inference-worker (HIGH)
- **Error count:** 23 in last hour
- **Most common:** GPU memory exhausted
- **Impact:** Inference jobs failing

### Service Relationships
inference-gateway → inference-platform-api → inference-worker
                                           ↘ tm-temporal-worker (OK)
```

---

## 2. Service Health Check

**User asks:** "How is inference-gateway doing?"

### Step 1: Get Operations Overview
```
signoz_get_service_top_operations(service='inference-gateway', timeRange='1h')
```

### Step 2: Check for Errors
```
signoz_search_logs_by_service(service='inference-gateway', severity='ERROR', timeRange='1h', limit=20)
```

### Step 3: Sample Traces
```
signoz_search_traces_by_service(service='inference-gateway', timeRange='1h', limit=10)
```

### Step 4: Output Format
```
## inference-gateway Health Report

### Performance (Last Hour)
| Operation | P99 Latency | Error Rate | Requests |
|-----------|-------------|------------|----------|
| POST /inference | 245ms | 0.3% | 12,450 |
| GET /status | 12ms | 0% | 8,320 |

### Recent Errors
- 3x "Connection refused to inference-worker:8080"
- 1x "Request timeout after 30s"

### Status: HEALTHY with minor issues
```

---

## 3. Alert Investigation

**User asks:** "Tell me about this alert"

### Step 1: Get Alert Details
```
signoz_get_alert(ruleId='<id>')
```

### Step 2: Check History
```
signoz_get_alert_history(ruleId='<id>', timeRange='24h')
```

### Step 3: Get Related Logs
```
signoz_get_logs_for_alert(alertId='<id>', timeRange='1h')
```

### Step 4: Output Format
```
## Alert: High Error Rate on inference-gateway

### Definition
- **Condition:** error_rate > 5% for 5 minutes
- **Current:** 7.2%
- **State:** FIRING

### Timeline
- 14:23 - First fired
- 14:35 - Resolved
- 15:12 - Fired again (current)

### Related Logs (around alert time)
- 15:10:23 ERROR inference-gateway: upstream timeout
- 15:11:45 ERROR inference-gateway: connection reset
```

---

## 4. Trace Deep Dive

**User asks:** "What happened with this request?" or "Show me the flow of trace X"

### Step 1: Get Full Trace
```
signoz_get_trace_details(traceId='<id>')
```

### Step 2: Get Hierarchy
```
signoz_get_trace_span_hierarchy(traceId='<id>')
```

### Step 3: Output Format
```
## Trace Analysis: abc123xyz

### Overview
- **Duration:** 2.3s (SLOW)
- **Services:** 4
- **Spans:** 12
- **Status:** ERROR

### Flow
coolify-traefik (2ms)
└── inference-gateway (45ms)
    ├── cookie-manager-service (12ms) ✓
    └── inference-platform-api (2.1s) ✗ SLOW
        └── inference-worker (1.9s) ✗ ERROR
            └── "GPU OOM: requested 8GB, available 2GB"

### Root Cause
inference-worker failed with GPU out of memory error
```

---

## 5. Performance Investigation

**User asks:** "Why is the API slow?"

### Step 1: Check Service Operations
```
signoz_get_service_top_operations(service='inference-platform-api', timeRange='1h')
```

### Step 2: Find Slow Traces
```
signoz_search_traces_by_service(
  service='inference-platform-api',
  minDuration=1000000000,  # 1 second in nanoseconds
  timeRange='1h',
  limit=20
)
```

### Step 3: Analyze Slow Traces
```
signoz_get_trace_span_hierarchy(traceId='<slowest_trace_id>')
```

### Step 4: Output Format
```
## Performance Analysis: inference-platform-api

### Latency Distribution (Last Hour)
- P50: 120ms
- P95: 890ms
- P99: 2.4s ← PROBLEM

### Slow Request Pattern
20 requests > 1s found. Common pattern:
- inference-platform-api → inference-worker calls
- Worker processing: avg 1.8s

### Bottleneck Identified
inference-worker is the bottleneck:
- GPU inference taking 1.5-2s
- Queue backlog during peak times
```

---

## 6. Queue Monitoring

**User asks:** "How is the ticket queue doing?"

### Step 1: Search Queue Metrics
```
signoz_search_metric_by_text(searchText='ticket_queue')
```

### Step 2: Check Dashboard
```
signoz_get_dashboard(uuid='01998318-be16-748c-9f65-611d1bfc9c74')
```

### Step 3: Look for Queue Errors
```
signoz_get_error_logs(timeRange='1h', limit=50)
# Filter for queue-related errors
```

### Step 4: Output Format
```
## Ticket Queue Status

### Metrics (Last Hour)
- **tickets_inserted:** 1,234
- **poll_iterations:** 5,678
- **iteration_errors:** 12 (0.2%)
- **queue_empty_iterations:** 890 (15.7%)

### Health: GOOD
- Low error rate
- Queue being consumed (not backing up)
```

---

## 7. Service Comparison

**User asks:** "Compare inference-gateway and inference-platform-api"

### Steps
```
signoz_get_service_top_operations(service='inference-gateway', timeRange='1h')
signoz_get_service_top_operations(service='inference-platform-api', timeRange='1h')
```

### Output Format
```
## Service Comparison (Last Hour)

| Metric | inference-gateway | inference-platform-api |
|--------|-------------------|------------------------|
| Total Requests | 18,842 | 147 |
| P99 Latency | 245ms | 890ms |
| Error Rate | 0.3% | 0.1% |
| Top Operation | POST /inference | ProcessInference |
```

---

## Output Guidelines

### Always Include
1. **Time context** - When the data is from
2. **Severity indicators** - CRITICAL/HIGH/MEDIUM/LOW or OK/WARNING/ERROR
3. **Service relationships** - How services connect
4. **Actionable insights** - Not just data dumps

### Formatting
- Use tables for comparisons
- Use ASCII diagrams for flows
- Bold important values
- Use ✓ and ✗ for status indicators
- Group by service when multiple are involved

### Severity Classification
- **CRITICAL:** User-facing services down (inference-gateway)
- **HIGH:** Core services affected (inference-platform-api, cookie-manager)
- **MEDIUM:** Background services affected (workers, langfuse)
- **LOW:** Non-critical services (session-viewer, admin)

---

## 8. Essential Alerts (Create in SigNoz UI)

These alerts should be created manually in SigNoz (Alerts → New Alert).

### Alert 1: High Error Rate - Critical Services
```yaml
Name: High Error Rate (Critical Services)
Severity: CRITICAL
Signal: Traces
Query:
  - A: count() where has_error = true AND service.name IN ('inference-platform-api', 'cookie-manager-service')
  - B: count() where service.name IN ('inference-platform-api', 'cookie-manager-service')
  - Formula: A/B
Condition: F1 > 0.01 (1%)
For: 5 minutes
Labels:
  - team: platform
  - priority: P1
```

### Alert 2: P99 Latency Spike
```yaml
Name: P99 Latency Spike (API)
Severity: HIGH
Signal: Traces
Query:
  - A: p99(duration_nano) where service.name = 'inference-platform-api'
Condition: A > 2000000000 (2 seconds in nanoseconds)
For: 10 minutes
Labels:
  - team: platform
  - priority: P2
```

### Alert 3: Gateway Timeout Errors
```yaml
Name: Gateway Timeouts (504)
Severity: HIGH
Signal: Traces
Query:
  - A: count() where response_status_code = 504
Condition: A > 5
For: 5 minutes
Labels:
  - team: platform
  - priority: P2
```

### Alert 4: Queue Processing Errors
```yaml
Name: Queue Processing Errors
Severity: MEDIUM
Signal: Metrics
Query (PromQL):
  rate({ticket_queue.iteration_errors}[5m])
Condition: > 0.1 (more than 6 errors/minute)
For: 5 minutes
Labels:
  - team: platform
  - priority: P3
```

### Alert 5: Auth Service Errors
```yaml
Name: Auth Service High Error Rate
Severity: HIGH
Signal: Traces
Query:
  - A: count() where has_error = true AND service.name = 'cookie-manager-service'
  - B: count() where service.name = 'cookie-manager-service'
  - Formula: A/B
Condition: F1 > 0.05 (5%)
For: 5 minutes
Labels:
  - team: platform
  - priority: P2
```

### Alert 6: Service Down (Zero Traffic)
```yaml
Name: Service Down (Zero Traffic)
Severity: CRITICAL
Signal: Traces
Query:
  - A: count() where service.name IN ('inference-platform-api', 'cookie-manager-service')
Condition: A = 0
For: 5 minutes
Labels:
  - team: platform
  - priority: P1
```

### Alert 7: Traefik High Error Rate
```yaml
Name: Traefik High Error Rate
Severity: HIGH
Signal: Metrics
Query (PromQL):
  100 * sum(rate({traefik_entrypoint_requests_total, code=~"5.."}[5m]))
  / sum(rate({traefik_entrypoint_requests_total}[5m]))
Condition: > 5 (5% 5xx errors)
For: 5 minutes
Labels:
  - team: infrastructure
  - priority: P2
```

### Notification Channels
Configure these in SigNoz Settings → Alert Channels:
- **Slack:** #platform-alerts for CRITICAL/HIGH
- **Email:** on-call@company.com for all severities
- **PagerDuty:** P1 incidents only
