# SigNoz MCP Tools Reference

## Services & Operations

### signoz_list_services
Lists all services within a time range.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| timeRange | No | Time range: '2h', '6h', '2d', '7d' |
| start | No | Start time in nanoseconds |
| end | No | End time in nanoseconds |

**Example:**
```
signoz_list_services(timeRange='6h')
```

### signoz_get_service_top_operations
Gets top operations for a service with performance metrics.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| service | Yes | Service name |
| timeRange | No | Time range: '2h', '6h', '2d', '7d' |
| start | No | Start time in nanoseconds |
| end | No | End time in nanoseconds |
| tags | No | JSON array of tags |

**Returns:** Operations with p99 latency, error rates, request counts

**Example:**
```
signoz_get_service_top_operations(service='inference-gateway', timeRange='1h')
```

---

## Alerts

### signoz_list_alerts
Lists all active alerts with status.

**Parameters:** None

**Returns:** All alert rules with current state

### signoz_get_alert
Gets details of a specific alert rule.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| ruleId | Yes | Alert rule ID |

**Returns:** Alert definition, thresholds, current state

### signoz_get_alert_history
Gets alert history timeline.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| ruleId | Yes | Alert rule ID |
| timeRange | No | Time range: '2h', '6h', '2d', '7d' |
| start | No | Start timestamp in milliseconds |
| end | No | End timestamp in milliseconds |
| offset | No | Pagination offset (default: 0) |
| limit | No | Result limit (default: 20) |
| order | No | Sort: 'asc' or 'desc' (default: 'asc') |

**Example:**
```
signoz_get_alert_history(ruleId='abc123', timeRange='24h')
```

---

## Logs

### signoz_list_log_views
Lists all saved log views.

**Returns:** Views with name, ID, description, query details

### signoz_get_log_view
Gets full details of a log view.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| viewId | Yes | Log view ID |

### signoz_get_logs_for_alert
Gets logs related to a specific alert.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| alertId | Yes | Alert rule ID |
| timeRange | No | Time range around alert (default: '1h') |
| limit | No | Max logs to return (default: 100) |

### signoz_get_error_logs
Gets ERROR/FATAL severity logs.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| timeRange | No | Time range: '2h', '6h', '2d', '7d' |
| start | No | Start time in milliseconds |
| end | No | End time in milliseconds |
| service | No | Service name filter |
| limit | No | Max logs (default: 100) |

**Example:**
```
signoz_get_error_logs(service='inference-gateway', timeRange='1h', limit=50)
```

### signoz_search_logs_by_service
Searches logs for a specific service.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| service | Yes | Service name |
| timeRange | No | Time range: '2h', '6h', '2d', '7d' |
| start | No | Start time in milliseconds |
| end | No | End time in milliseconds |
| severity | No | DEBUG, INFO, WARN, ERROR, FATAL |
| searchText | No | Text to search in log body |
| limit | No | Max logs (default: 100) |

**Example:**
```
signoz_search_logs_by_service(service='inference-worker', severity='ERROR', searchText='timeout')
```

---

## Traces

### signoz_get_trace_field_values
Gets available field values for filtering.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| fieldName | Yes | Field name (e.g., 'service.name', 'http.method') |
| searchText | No | Filter values by text |

### signoz_search_traces_by_service
Searches traces for a service.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| service | Yes | Service name |
| timeRange | No | Time range: '2h', '6h', '2d', '7d' |
| start | No | Start time in milliseconds |
| end | No | End time in milliseconds |
| operation | No | Operation name filter |
| error | No | Filter by error (true/false) |
| minDuration | No | Min duration in nanoseconds |
| maxDuration | No | Max duration in nanoseconds |
| limit | No | Max traces (default: 100) |

**Example:**
```
signoz_search_traces_by_service(service='inference-gateway', error=true, timeRange='1h')
```

### signoz_get_trace_details
Gets full trace with all spans.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| traceId | Yes | Trace ID |
| timeRange | No | Time range: '2h', '6h', '2d', '7d' |
| start | No | Start time in milliseconds |
| end | No | End time in milliseconds |
| includeSpans | No | Include span details (default: true) |

### signoz_get_trace_error_analysis
Analyzes error patterns in traces.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| timeRange | No | Time range: '2h', '6h', '2d', '7d' |
| start | No | Start time in milliseconds |
| end | No | End time in milliseconds |
| service | No | Service name filter |

**Returns:** Traces with errors, patterns, affected services

### signoz_get_trace_span_hierarchy
Gets span relationships and hierarchy.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| traceId | Yes | Trace ID |
| timeRange | No | Time range: '2h', '6h', '2d', '7d' |
| start | No | Start time in milliseconds |
| end | No | End time in milliseconds |

**Returns:** Parent-child span relationships, call flow visualization

---

## Metrics

### signoz_list_metric_keys
Lists all available metric keys.

**Parameters:** None

**Returns:** All metric names from SigNoz

### signoz_search_metric_by_text
Searches metrics by text.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| searchText | Yes | Text to search for |

**Example:**
```
signoz_search_metric_by_text(searchText='ticket_queue')
```

---

## Dashboards

### signoz_list_dashboards
Lists all dashboards.

**Returns:** Dashboard summaries (name, UUID, description, tags)

### signoz_get_dashboard
Gets complete dashboard configuration.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| uuid | Yes | Dashboard UUID |

**Example:**
```
signoz_get_dashboard(uuid='01998318-be16-748c-9f65-611d1bfc9c74')
```

### signoz_create_dashboard
Creates a new dashboard.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| title | Yes | Dashboard name |
| description | No | Short summary |
| tags | No | List of tags |
| layout | Yes | Widget positioning grid |
| variables | No | Query variables |
| widgets | Yes | Widget configurations |

**Warning:** Requires full dashboard JSON, can consume large context

### signoz_update_dashboard
Updates an existing dashboard.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| uuid | Yes | Dashboard UUID |
| title | Yes | Dashboard name |
| description | No | Short summary |
| tags | No | List of tags |
| layout | Yes | Full widget positioning grid |
| variables | No | Query variables |
| widgets | Yes | Complete widget set |

---

## Query Builder

### signoz_query_helper
Helper for building SigNoz queries.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| signal | No | Signal type: traces, logs, metrics |
| query_type | No | Help type: fields, structure, examples, all |

**Returns:** Guidance on fields, signal types, query structure

### signoz_execute_builder_query
Executes a raw Query Builder v5 query.

**Parameters:**
| Name | Required | Description |
|------|----------|-------------|
| query | Yes | Complete Query Builder v5 JSON object |

**Use for:** Complex queries not covered by other tools

---

## Time Format Reference

| Format | Description |
|--------|-------------|
| '30m' | Last 30 minutes |
| '1h' | Last 1 hour |
| '2h' | Last 2 hours |
| '6h' | Last 6 hours |
| '24h' | Last 24 hours |
| '2d' | Last 2 days |
| '7d' | Last 7 days |

Most tools default to 6 hours if no time range specified.
