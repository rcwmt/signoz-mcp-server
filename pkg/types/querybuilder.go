package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// QueryPayload is struct used as payload the Query Builder v5 JSON schema
type QueryPayload struct {
	SchemaVersion  string         `json:"schemaVersion"`
	Start          int64          `json:"start"`
	End            int64          `json:"end"`
	RequestType    string         `json:"requestType"`
	CompositeQuery CompositeQuery `json:"compositeQuery"`
	FormatOptions  FormatOptions  `json:"formatOptions"`
	Variables      map[string]any `json:"variables"`
}

type CompositeQuery struct {
	Queries []Query `json:"queries"`
}

// Each envelope has its own spec; SQL must never acquire builder fields.
type Query struct {
	Type string `json:"type"`
	Spec any    `json:"spec"`
}

type ClickHouseSQLSpec struct {
	Name     string `json:"name"`
	Query    string `json:"query"`
	Disabled bool   `json:"disabled,omitempty"`
	Legend   string `json:"legend,omitempty"`
}

func (q *QueryPayload) UnmarshalJSON(data []byte) error {
	type payload QueryPayload
	return decodeQueryJSON(data, (*payload)(q))
}

func decodeQueryJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func (q *Query) UnmarshalJSON(data []byte) error {
	var envelope struct {
		Type string          `json:"type"`
		Spec json.RawMessage `json:"spec"`
	}
	if err := decodeQueryJSON(data, &envelope); err != nil {
		return err
	}
	q.Type = envelope.Type
	switch q.Type {
	case "builder_query":
		var spec QuerySpec
		if err := decodeQueryJSON(envelope.Spec, &spec); err != nil {
			return fmt.Errorf("invalid builder_query spec: %w", err)
		}
		q.Spec = spec
	case "clickhouse_sql":
		var spec ClickHouseSQLSpec
		if err := decodeQueryJSON(envelope.Spec, &spec); err != nil {
			return fmt.Errorf("invalid clickhouse_sql spec: %w", err)
		}
		q.Spec = spec
	default:
		return fmt.Errorf("unsupported query type %q", q.Type)
	}
	return nil
}

type QuerySpec struct {
	Name         string        `json:"name"`
	Signal       string        `json:"signal"`
	StepInterval *int64        `json:"stepInterval,omitempty"`
	Disabled     bool          `json:"disabled"`
	Filter       *Filter       `json:"filter,omitempty"`
	Limit        int           `json:"limit"`
	Offset       int           `json:"offset"`
	Order        []Order       `json:"order"`
	Having       Having        `json:"having"`
	SelectFields []SelectField `json:"selectFields"`
	Aggregations []any         `json:"aggregations,omitempty"`
	GroupBy      []SelectField `json:"groupBy,omitempty"`
}

type Order struct {
	Key       Key    `json:"key"`
	Direction string `json:"direction"`
}

type Key struct {
	Name string `json:"name"`
}

type Having struct {
	Expression string `json:"expression"`
}

type Filter struct {
	Expression string `json:"expression"`
}

type SelectField struct {
	Name          string `json:"name"`
	FieldDataType string `json:"fieldDataType"`
	Signal        string `json:"signal"`
	FieldContext  string `json:"fieldContext,omitempty"`
}

type FormatOptions struct {
	FormatTableResultForUI bool `json:"formatTableResultForUI"`
	FillGaps               bool `json:"fillGaps"`
}

// Validate performs necessary validation for required fields
// this indirectly helps LLMs to build right payload.
// if there is an error LLM checks the error and fix.
func (q *QueryPayload) Validate() error {
	if q.SchemaVersion == "" {
		q.SchemaVersion = "v1"
	}

	if q.Start == 0 || q.End == 0 {
		return fmt.Errorf("missing start or end timestamp")
	}
	if len(q.CompositeQuery.Queries) == 0 {
		return fmt.Errorf("missing or empty compositeQuery.queries")
	}

	if q.RequestType != "" && q.RequestType != "raw" && q.RequestType != "trace" && q.RequestType != "time_series" && q.RequestType != "scalar" {
		return fmt.Errorf("unsupported requestType %q", q.RequestType)
	}
	if q.RequestType == "" {
		defaultSignal := ""
		for _, query := range q.CompositeQuery.Queries {
			if query.Type != "builder_query" {
				continue
			}
			spec, ok := query.Spec.(QuerySpec)
			if !ok {
				return fmt.Errorf("invalid builder spec")
			}
			if defaultSignal != "" && defaultSignal != spec.Signal {
				return fmt.Errorf("mixed signals require an explicit requestType")
			}
			defaultSignal = spec.Signal
		}
		q.RequestType = "raw"
		if defaultSignal == "metrics" {
			q.RequestType = "time_series"
		}
	}
	for i, query := range q.CompositeQuery.Queries {
		if query.Type == "clickhouse_sql" {
			spec, ok := query.Spec.(ClickHouseSQLSpec)
			if !ok || strings.TrimSpace(spec.Name) == "" || strings.TrimSpace(spec.Query) == "" {
				return fmt.Errorf("query at position %d: SQL requires name and query", i+1)
			}
			continue
		}
		if query.Type != "builder_query" {
			return fmt.Errorf("unsupported query type %q", query.Type)
		}
		spec, ok := query.Spec.(QuerySpec)
		if !ok {
			return fmt.Errorf("query at position %d: invalid builder spec", i+1)
		}
		switch spec.Signal {
		case "metrics":
			if q.RequestType != "time_series" && q.RequestType != "scalar" {
				return fmt.Errorf("%s: metrics require time_series or scalar", spec.Name)
			}
		case "traces":
		case "logs":
			if q.RequestType == "trace" {
				return fmt.Errorf("%s: logs do not support requestType trace", spec.Name)
			}
		default:
			return fmt.Errorf("%s: unknown signal type %q", spec.Name, spec.Signal)
		}
		if q.RequestType == "time_series" {
			if spec.StepInterval == nil || *spec.StepInterval <= 0 {
				step := int64(60)
				spec.StepInterval = &step
			}
		} else {
			spec.StepInterval = nil
		}
		q.CompositeQuery.Queries[i].Spec = spec
	}
	return nil
}

// BuildQueryHelpText provides guidance for building SigNoz queries
func BuildQueryHelpText(signal, queryType string) string {
	switch signal {
	case "traces":
		return buildTracesHelpText(queryType)
	case "logs":
		return buildLogsHelpText(queryType)
	case "metrics":
		return buildMetricsHelpText(queryType)
	default:
		return buildGeneralHelpText(queryType)
	}
}

func buildTracesHelpText(queryType string) string {
	switch queryType {
	case "fields":
		return `Available trace fields:
- traceID (trace identifier)
- spanID (span identifier)
- parentSpanID (parent span identifier)
- service.name (resource context)
- name (operation/span name)
- durationNano (span duration in nanoseconds)
- timestamp (span timestamp)
- hasError (boolean, true if span has error)
- statusCode (HTTP status code)
- statusCodeString (status code as string)
- httpMethod (HTTP method)
- httpUrl (HTTP URL)
- spanKind (span kind: client, server, producer, consumer)
- rpcMethod (RPC method name)
- kind (span kind as integer)
- resource attributes: service.version, deployment.environment, etc.
- span attributes: http.method, http.status_code, db.system, etc.`
	case "structure":
		return `Trace query structure (returns comprehensive trace data):
{
  "schemaVersion": "v1",
  "start": 1704067200000,
  "end": 1758758400000,
  "requestType": "raw",
  "compositeQuery": {
    "queries": [{
      "type": "builder_query",
      "spec": {
        "name": "A",
        "signal": "traces",
        "disabled": false,
        "limit": 100,
        "offset": 0,
        "order": [{"key": {"name": "timestamp"}, "direction": "desc"}],
        "having": {"expression": ""},
        "selectFields": [
          {"name": "traceID", "fieldDataType": "string", "signal": "traces"},
          {"name": "spanID", "fieldDataType": "string", "signal": "traces"},
          {"name": "parentSpanID", "fieldDataType": "string", "signal": "traces"},
          {"name": "service.name", "fieldDataType": "string", "signal": "traces", "fieldContext": "resource"},
          {"name": "name", "fieldDataType": "string", "signal": "traces"},
          {"name": "durationNano", "fieldDataType": "int64", "signal": "traces"},
          {"name": "timestamp", "fieldDataType": "string", "signal": "traces"},
          {"name": "hasError", "fieldDataType": "bool", "signal": "traces"},
          {"name": "statusCode", "fieldDataType": "string", "signal": "traces"},
          {"name": "httpMethod", "fieldDataType": "string", "signal": "traces"},
          {"name": "spanKind", "fieldDataType": "string", "signal": "traces"}
        ]
      }
    }]
  },
  "formatOptions": {"formatTableResultForUI": false, "fillGaps": false},
  "variables": {}
}`
	case "examples":
		return `Example trace queries (all return comprehensive trace data):

1. Recent slow traces:
{
  "schemaVersion": "v1",
  "start": 1704067200000,
  "end": 1758758400000,
  "requestType": "raw",
  "compositeQuery": {
    "queries": [{
      "type": "builder_query",
      "spec": {
        "name": "A",
        "signal": "traces",
        "disabled": false,
        "filter": {"expression": "durationNano > 1000000000"},
        "limit": 100,
        "offset": 0,
        "order": [{"key": {"name": "durationNano"}, "direction": "desc"}],
        "having": {"expression": ""}
      }
    }]
  },
  "formatOptions": {"formatTableResultForUI": false, "fillGaps": false},
  "variables": {}
}

2. Error traces:
{
  "schemaVersion": "v1",
  "start": 1704067200000,
  "end": 1758758400000,
  "requestType": "raw",
  "compositeQuery": {
    "queries": [{
      "type": "builder_query",
      "spec": {
        "name": "A",
        "signal": "traces",
        "disabled": false,
        "filter": {"expression": "hasError = true"},
        "limit": 100,
        "offset": 0,
        "order": [{"key": {"name": "timestamp"}, "direction": "desc"}],
        "having": {"expression": ""}
      }
    }]
  },
  "formatOptions": {"formatTableResultForUI": false, "fillGaps": false},
  "variables": {}
}

3. Specific trace by ID:
{
  "schemaVersion": "v1",
  "start": 1704067200000,
  "end": 1758758400000,
  "requestType": "raw",
  "compositeQuery": {
    "queries": [{
      "type": "builder_query",
      "spec": {
        "name": "A",
        "signal": "traces",
        "disabled": false,
        "filter": {"expression": "traceID = '7a2740c79ab31eea9534951a6fd5b2b6'"},
        "limit": 1000,
        "offset": 0,
        "order": [{"key": {"name": "timestamp"}, "direction": "desc"}],
        "having": {"expression": ""}
      }
    }]
  },
  "formatOptions": {"formatTableResultForUI": false, "fillGaps": false},
  "variables": {}
}`
	default:
		return buildTracesHelpText("fields") + "\n\n" + buildTracesHelpText("structure") + "\n\n" + buildTracesHelpText("examples")
	}
}

func buildLogsHelpText(queryType string) string {
	switch queryType {
	case "fields":
		return `Available log fields:
- timestamp (log timestamp)
- severity_text (log level: DEBUG, INFO, WARN, ERROR, FATAL)
- body (log message)
- service.name (resource attribute)
- trace_id (trace identifier if linked)
- span_id (span identifier if linked)
- resource attributes: service.version, deployment.environment, etc.
- log attributes: custom key-value pairs`
	case "structure":
		return `Log query structure:
{
  "schemaVersion": "v1",
  "start": 1704067200000,
  "end": 1758758400000,
  "requestType": "raw",
  "compositeQuery": {
    "queries": [{
      "type": "builder_query",
      "spec": {
        "name": "A",
        "signal": "logs",
        "disabled": false,
        "limit": 10,
        "offset": 0,
        "order": [{"key": {"name": "timestamp"}, "direction": "desc"}],
        "having": {"expression": ""},
        "selectFields": [
          {"name": "timestamp", "fieldDataType": "string", "signal": "logs"},
          {"name": "severity_text", "fieldDataType": "string", "signal": "logs"},
          {"name": "body", "fieldDataType": "string", "signal": "logs"},
          {"name": "service.name", "fieldDataType": "string", "signal": "logs", "fieldContext": "resource"}
        ]
      }
    }]
  },
  "formatOptions": {"formatTableResultForUI": false, "fillGaps": false},
  "variables": {}
}`
	case "examples":
		return `Example log queries:

1. Error logs:
{
  "schemaVersion": "v1",
  "start": 1704067200000,
  "end": 1758758400000,
  "requestType": "raw",
  "compositeQuery": {
    "queries": [{
      "type": "builder_query",
      "spec": {
        "name": "A",
        "signal": "logs",
        "disabled": false,
        "filter": {"expression": "severity_text = 'ERROR'"},
        "limit": 10,
        "offset": 0,
        "order": [{"key": {"name": "timestamp"}, "direction": "desc"}],
        "having": {"expression": ""},
        "selectFields": [
          {"name": "timestamp", "fieldDataType": "string", "signal": "logs"},
          {"name": "severity_text", "fieldDataType": "string", "signal": "logs"},
          {"name": "body", "fieldDataType": "string", "signal": "logs"},
          {"name": "service.name", "fieldDataType": "string", "signal": "logs", "fieldContext": "resource"}
        ]
      }
    }]
  },
  "formatOptions": {"formatTableResultForUI": false, "fillGaps": false},
  "variables": {}
}

2. Service-specific logs with text search:
{
  "schemaVersion": "v1",
  "start": 1704067200000,
  "end": 1758758400000,
  "requestType": "raw",
  "compositeQuery": {
    "queries": [{
      "type": "builder_query",
      "spec": {
        "name": "A",
        "signal": "logs",
        "disabled": false,
        "filter": {"expression": "service.name in ['my-service'] AND body CONTAINS 'error'"},
        "limit": 10,
        "offset": 0,
        "order": [{"key": {"name": "timestamp"}, "direction": "desc"}],
        "having": {"expression": ""},
        "selectFields": [
          {"name": "timestamp", "fieldDataType": "string", "signal": "logs"},
          {"name": "severity_text", "fieldDataType": "string", "signal": "logs"},
          {"name": "body", "fieldDataType": "string", "signal": "logs"},
          {"name": "service.name", "fieldDataType": "string", "signal": "logs", "fieldContext": "resource"}
        ]
      }
    }]
  },
  "formatOptions": {"formatTableResultForUI": false, "fillGaps": false},
  "variables": {}
}`
	default:
		return buildLogsHelpText("fields") + "\n\n" + buildLogsHelpText("structure") + "\n\n" + buildLogsHelpText("examples")
	}
}

func buildMetricsHelpText(queryType string) string {
	switch queryType {
	case "fields":
		return `Available metric fields:
- timestamp (metric timestamp)
- value (metric value)
- metric_name (name of the metric)
- service.name (resource attribute)
- resource attributes: service.version, deployment.environment, etc.
- metric attributes: custom labels and tags`
	case "structure":
		return `Metric query structure (aggregations field is REQUIRED for metrics queries):

{
  "schemaVersion": "v1",
  "start": 1704067200000,
  "end": 1758758400000,
  "requestType": "time_series",
  "compositeQuery": {
    "queries": [{
      "type": "builder_query",
      "spec": {
        "name": "A",
        "signal": "metrics",
        "disabled": false,
        "limit": 10,
        "offset": 0,
        "order": [{"key": {"name": "0"}, "direction": "desc"}],
        "having": {"expression": ""},
        "selectFields": [
          {"name": "value", "fieldDataType": "float64", "signal": "metrics"},
          {"name": "timestamp", "fieldDataType": "string", "signal": "metrics"}
        ],
        "stepInterval": 60,
        "aggregations": [{
          "metricName": "cpu_usage",
          "timeAggregation": "avg",
          "spaceAggregation": "avg",
          "temporality": "Unspecified"
        }]
      }
    }]
  },
  "formatOptions": {"formatTableResultForUI": false, "fillGaps": false},
  "variables": {}
}

Note: The aggregations field is REQUIRED for metrics queries. Always include it explicitly with the metricName.`
	case "examples":
		return `Example metric queries:

1. CPU usage over time (with explicit aggregations):
{
  "schemaVersion": "v1",
  "start": 1704067200000,
  "end": 1758758400000,
  "requestType": "time_series",
  "compositeQuery": {
    "queries": [{
      "type": "builder_query",
      "spec": {
        "name": "A",
        "signal": "metrics",
        "disabled": false,
        "limit": 100,
        "offset": 0,
        "order": [{"key": {"name": "0"}, "direction": "asc"}],
        "having": {"expression": ""},
        "selectFields": [
          {"name": "value", "fieldDataType": "float64", "signal": "metrics"},
          {"name": "timestamp", "fieldDataType": "string", "signal": "metrics"}
        ],
        "stepInterval": 60,
        "aggregations": [{
          "metricName": "cpu_usage",
          "timeAggregation": "avg",
          "spaceAggregation": "avg",
          "temporality": "Unspecified"
        }]
      }
    }]
  },
  "formatOptions": {"formatTableResultForUI": false, "fillGaps": false},
  "variables": {}
}

2. Memory usage with rate aggregation:
{
  "schemaVersion": "v1",
  "start": 1704067200000,
  "end": 1758758400000,
  "requestType": "time_series",
  "compositeQuery": {
    "queries": [{
      "type": "builder_query",
      "spec": {
        "name": "A",
        "signal": "metrics",
        "disabled": false,
        "limit": 100,
        "offset": 0,
        "order": [{"key": {"name": "0"}, "direction": "desc"}],
        "having": {"expression": ""},
        "selectFields": [
          {"name": "value", "fieldDataType": "float64", "signal": "metrics"},
          {"name": "timestamp", "fieldDataType": "string", "signal": "metrics"}
        ],
        "stepInterval": 60,
        "aggregations": [{
          "metricName": "memory_usage",
          "timeAggregation": "rate",
          "spaceAggregation": "sum",
          "temporality": "Cumulative"
        }]
      }
    }]
  },
  "formatOptions": {"formatTableResultForUI": false, "fillGaps": false},
  "variables": {}
}

IMPORTANT: The "aggregations" field is REQUIRED for metrics queries. Always include it explicitly with the metricName.`
	default:
		return buildMetricsHelpText("fields") + "\n\n" + buildMetricsHelpText("structure") + "\n\n" + buildMetricsHelpText("examples")
	}
}

func buildGeneralHelpText(queryType string) string {
	return `SigNoz Query Builder v5 Help

Available signals: traces, logs, metrics

Common query parameters:
- schemaVersion: Always "v1"
- start/end: Timestamps in milliseconds since epoch
- requestType: "raw" for point-in-time data, "time_series" for time-series data
- compositeQuery.queries[].spec.signal: "traces", "logs", or "metrics"
- compositeQuery.queries[].spec.limit: Number of results to return
- compositeQuery.queries[].spec.order: Sort order for results
- compositeQuery.queries[].spec.selectFields: Fields to include in results
- compositeQuery.queries[].spec.having: Filter expression for aggregated results

Use signoz_query_helper with specific signal type for detailed field information.`
}

// BuildLogsQueryPayload creates a QueryPayload for logs queries
func BuildLogsQueryPayload(startTime, endTime int64, filterExpression string, limit int, offset int) *QueryPayload {
	return &QueryPayload{
		SchemaVersion: "v1",
		Start:         startTime,
		End:           endTime,
		RequestType:   "raw",
		CompositeQuery: CompositeQuery{
			Queries: []Query{
				{
					Type: "builder_query",
					Spec: QuerySpec{
						Name:     "A",
						Signal:   "logs",
						Disabled: false,
						Filter:   &Filter{Expression: filterExpression},
						Limit:    limit,
						Offset:   offset,
						Order: []Order{
							{Key: Key{Name: "timestamp"}, Direction: "desc"},
						},
						Having: Having{Expression: ""},
						SelectFields: []SelectField{
							{Name: "timestamp", FieldDataType: "string", Signal: "logs"},
							{Name: "severity_text", FieldDataType: "string", Signal: "logs"},
							{Name: "body", FieldDataType: "string", Signal: "logs"},
							{Name: "service.name", FieldDataType: "string", Signal: "logs", FieldContext: "resource"},
						},
					},
				},
			},
		},
		FormatOptions: FormatOptions{
			FormatTableResultForUI: false,
			FillGaps:               false,
		},
		Variables: map[string]any{},
	}
}

func BuildTracesQueryPayload(startTime, endTime int64, filterExpression string, limit int) *QueryPayload {
	return &QueryPayload{
		SchemaVersion: "v1",
		Start:         startTime,
		End:           endTime,
		RequestType:   "raw",
		CompositeQuery: CompositeQuery{
			Queries: []Query{
				{
					Type: "builder_query",
					Spec: QuerySpec{
						Name:     "A",
						Signal:   "traces",
						Disabled: false,
						Filter:   &Filter{Expression: filterExpression},
						Limit:    limit,
						Offset:   0,
						Order: []Order{
							{Key: Key{Name: "timestamp"}, Direction: "desc"},
						},
						Having: Having{Expression: ""},
						SelectFields: []SelectField{
							{Name: "traceID", FieldDataType: "string", Signal: "traces"},
							{Name: "spanID", FieldDataType: "string", Signal: "traces"},
							{Name: "parentSpanID", FieldDataType: "string", Signal: "traces"},
							{Name: "service.name", FieldDataType: "string", Signal: "traces", FieldContext: "resource"},
							{Name: "name", FieldDataType: "string", Signal: "traces"},
							{Name: "durationNano", FieldDataType: "int64", Signal: "traces"},
							{Name: "timestamp", FieldDataType: "string", Signal: "traces"},
							{Name: "hasError", FieldDataType: "bool", Signal: "traces"},
							{Name: "statusCode", FieldDataType: "string", Signal: "traces"},
							{Name: "statusCodeString", FieldDataType: "string", Signal: "traces"},
							{Name: "httpMethod", FieldDataType: "string", Signal: "traces"},
							{Name: "httpUrl", FieldDataType: "string", Signal: "traces"},
							{Name: "spanKind", FieldDataType: "string", Signal: "traces"},
							{Name: "rpcMethod", FieldDataType: "string", Signal: "traces"},
							{Name: "kind", FieldDataType: "int32", Signal: "traces"},
						},
					},
				},
			},
		},
		FormatOptions: FormatOptions{
			FormatTableResultForUI: false,
			FillGaps:               false,
		},
		Variables: map[string]any{},
	}
}
