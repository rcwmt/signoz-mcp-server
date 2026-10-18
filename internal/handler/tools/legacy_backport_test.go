package tools

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	signozclient "github.com/SigNoz/signoz-mcp-server/internal/client"
	"github.com/SigNoz/signoz-mcp-server/pkg/types"
	"github.com/SigNoz/signoz-mcp-server/pkg/util"
)

// The real HTTP client reaches only this private Unix socket, never a shared host port.
func legacyConsumer(t *testing.T, receiver http.HandlerFunc) *server.MCPServer {
	t.Helper()
	dir, err := os.MkdirTemp("", "signoz-mcp-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "backend.sock")
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	backend := &http.Server{Handler: receiver, ReadHeaderTimeout: time.Second}
	go func() { _ = backend.Serve(listener) }()
	t.Cleanup(func() { _ = backend.Close() })
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	previous := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: transport, Timeout: 2 * time.Second}
	t.Cleanup(func() { http.DefaultClient = previous; transport.CloseIdleConnections() })
	logger := zap.NewNop()
	h := NewHandler(logger, signozclient.NewClient(logger, "http://offline.invalid", "default-fixture-key"), "http://offline.invalid")
	s := server.NewMCPServer("legacy-offline", "test", server.WithToolCapabilities(false))
	h.RegisterQueryBuilderV5Handlers(s)
	h.RegisterDashboardHandlers(s)
	h.RegisterAlertsHandlers(s)
	return s
}

func legacyRPC(t *testing.T, s *server.MCPServer, method string, params any) json.RawMessage {
	t.Helper()
	request, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	require.NoError(t, err)
	// Exercise the existing per-request key consumer, not a replacement auth path.
	response := s.HandleMessage(util.SetAPIKey(context.Background(), "request-fixture-key"), request)
	encoded, err := json.Marshal(response)
	require.NoError(t, err)
	var wire struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	require.NoError(t, json.Unmarshal(encoded, &wire))
	require.Empty(t, wire.Error, string(encoded))
	require.NotEmpty(t, wire.Result)
	return wire.Result
}

func legacyCall(t *testing.T, s *server.MCPServer, name string, args any) mcp.CallToolResult {
	t.Helper()
	var result mcp.CallToolResult
	require.NoError(t, json.Unmarshal(legacyRPC(t, s, "tools/call", map[string]any{"name": name, "arguments": args}), &result))
	return result
}

func legacyText(t *testing.T, result mcp.CallToolResult) string {
	t.Helper()
	require.Len(t, result.Content, 1)
	text, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	return text.Text
}

func TestLegacyBackportQueryConsumer(t *testing.T) {
	for _, mode := range []string{"scalar", "time_series"} {
		t.Run("grouped_"+mode, func(t *testing.T) {
			var calls atomic.Int32
			s := legacyConsumer(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/api/v5/query_range", r.URL.Path)
				require.Equal(t, "request-fixture-key", r.Header.Get("SIGNOZ-API-KEY"))
				var got struct {
					RequestType    string `json:"requestType"`
					CompositeQuery struct {
						Queries []struct {
							Type string `json:"type"`
							Spec struct {
								Signal       string              `json:"signal"`
								GroupBy      []types.SelectField `json:"groupBy"`
								StepInterval *int64              `json:"stepInterval"`
								Aggregations []any               `json:"aggregations"`
							} `json:"spec"`
						} `json:"queries"`
					} `json:"compositeQuery"`
				}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
				require.Equal(t, mode, got.RequestType)
				require.Len(t, got.CompositeQuery.Queries, 3)
				for i, signal := range []string{"logs", "traces", "metrics"} {
					query := got.CompositeQuery.Queries[i]
					require.Equal(t, "builder_query", query.Type)
					require.Equal(t, signal, query.Spec.Signal)
					require.Equal(t, []types.SelectField{{Name: "service.name", FieldDataType: "string", Signal: signal, FieldContext: "resource"}}, query.Spec.GroupBy)
					require.Len(t, query.Spec.Aggregations, 1)
					if mode == "time_series" {
						require.NotNil(t, query.Spec.StepInterval)
						require.Equal(t, int64(30), *query.Spec.StepInterval)
					} else {
						require.Nil(t, query.Spec.StepInterval)
					}
				}
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, "offline rejection; no query executed")
			})
			queries := []any{}
			for _, signal := range []string{"logs", "traces", "metrics"} {
				spec := map[string]any{"name": signal, "signal": signal, "groupBy": []any{map[string]any{"name": "service.name", "fieldDataType": "string", "signal": signal, "fieldContext": "resource"}}, "aggregations": []any{map[string]any{"expression": "count()"}}}
				if mode == "time_series" {
					spec["stepInterval"] = 30
				}
				queries = append(queries, map[string]any{"type": "builder_query", "spec": spec})
			}
			result := legacyCall(t, s, "signoz_execute_builder_query", map[string]any{"query": map[string]any{"start": 1700000000000, "end": 1700003600000, "requestType": mode, "compositeQuery": map[string]any{"queries": queries}}})
			require.True(t, result.IsError)
			require.Contains(t, legacyText(t, result), "offline rejection; no query executed")
			require.Equal(t, int32(1), calls.Load())
		})
	}

	t.Run("SQL", func(t *testing.T) {
		var calls atomic.Int32
		s := legacyConsumer(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/api/v5/query_range", r.URL.Path)
			require.Equal(t, "request-fixture-key", r.Header.Get("SIGNOZ-API-KEY"))
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "raw", body["requestType"])
			query := body["compositeQuery"].(map[string]any)["queries"].([]any)[0].(map[string]any)
			require.Equal(t, "clickhouse_sql", query["type"])
			require.Equal(t, map[string]any{"name": "A", "query": "SELECT 1", "legend": "offline", "disabled": true}, query["spec"])
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "offline rejection; SQL not executed")
		})
		result := legacyCall(t, s, "signoz_execute_builder_query", map[string]any{"query": map[string]any{"start": 1, "end": 2, "compositeQuery": map[string]any{"queries": []any{map[string]any{"type": "clickhouse_sql", "spec": map[string]any{"name": "A", "query": "SELECT 1", "legend": "offline", "disabled": true}}}}}})
		require.Equal(t, int32(1), calls.Load())
		require.True(t, result.IsError)
		require.Contains(t, legacyText(t, result), "SQL not executed")
	})

	t.Run("fail_closed", func(t *testing.T) {
		var calls atomic.Int32
		s := legacyConsumer(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(http.StatusTeapot) })
		for name, query := range map[string]string{
			"SQL signal":        `{"start":1,"end":2,"compositeQuery":{"queries":[{"type":"clickhouse_sql","spec":{"name":"A","query":"SELECT 1","signal":"logs"}}]}}`,
			"missing SQL":       `{"start":1,"end":2,"compositeQuery":{"queries":[{"type":"clickhouse_sql","spec":{"name":"A"}}]}}`,
			"unknown envelope":  `{"start":1,"end":2,"compositeQuery":{"queries":[{"type":"promql","spec":{"name":"A","query":"up"}}]}}`,
			"unknown signal":    `{"start":1,"end":2,"compositeQuery":{"queries":[{"type":"builder_query","spec":{"signal":"unknown"}}]}}`,
			"unknown field":     `{"start":1,"end":2,"compositeQuery":{"queries":[{"type":"builder_query","spec":{"signal":"logs","unsupported":1}}]}}`,
			"unknown mode":      `{"start":1,"end":2,"requestType":"unknown","compositeQuery":{"queries":[{"type":"builder_query","spec":{"signal":"logs"}}]}}`,
			"incompatible mode": `{"start":1,"end":2,"requestType":"raw","compositeQuery":{"queries":[{"type":"builder_query","spec":{"signal":"metrics"}}]}}`,
			"missing start":     `{"end":2,"compositeQuery":{"queries":[{"type":"clickhouse_sql","spec":{"name":"A","query":"SELECT 1"}}]}}`,
		} {
			t.Run(name, func(t *testing.T) {
				var q map[string]any
				require.NoError(t, json.Unmarshal([]byte(query), &q))
				result := legacyCall(t, s, "signoz_execute_builder_query", map[string]any{"query": q})
				require.True(t, result.IsError)
			})
		}
		require.Zero(t, calls.Load(), "unsupported requests must not reach the backend")
	})

	t.Run("existing_typed_builders", func(t *testing.T) {
		var calls atomic.Int32
		s := legacyConsumer(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/api/v5/query_range", r.URL.Path)
			var body types.QueryPayload
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "raw", body.RequestType)
			require.Len(t, body.CompositeQuery.Queries, 1)
			w.WriteHeader(http.StatusBadRequest)
		})
		for _, payload := range []*types.QueryPayload{types.BuildLogsQueryPayload(1, 2, "", 5, 0), types.BuildTracesQueryPayload(1, 2, "", 5)} {
			data, err := json.Marshal(payload)
			require.NoError(t, err)
			var q map[string]any
			require.NoError(t, json.Unmarshal(data, &q))
			require.True(t, legacyCall(t, s, "signoz_execute_builder_query", map[string]any{"query": q}).IsError)
		}
		require.Equal(t, int32(2), calls.Load())
	})
}

func TestLegacyBackportDashboardAndCapabilities(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNotImplemented} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			body := `{"status":"success","data":{"id":"offline","source":"local","spec":{"panels":[{"kind":"TextPanel","spec":{"content":"untouched","queries":null}}]}}}`
			if status != http.StatusOK {
				body = `{"status":"error","errorType":"dashboard_deprecated","error":"offline error"}`
			}
			s := legacyConsumer(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "/api/v2/dashboards/offline%2Fdashboard%3Fx%23y", r.URL.EscapedPath())
				require.Equal(t, "request-fixture-key", r.Header.Get("SIGNOZ-API-KEY"))
				w.WriteHeader(status)
				_, _ = io.WriteString(w, body)
			})
			result := legacyCall(t, s, "signoz_get_dashboard", map[string]any{"uuid": "offline/dashboard?x#y"})
			require.Equal(t, int32(1), calls.Load())
			require.Equal(t, status != http.StatusOK, result.IsError)
			if status == http.StatusOK {
				require.Equal(t, body, legacyText(t, result))
			} else {
				require.Contains(t, legacyText(t, result), "unexpected status 501")
				require.Contains(t, legacyText(t, result), "offline error")
			}
			require.True(t, legacyCall(t, s, "signoz_get_dashboard", map[string]any{"id": "offline"}).IsError)
			require.Equal(t, int32(1), calls.Load(), "uuid remains the accepted schema; id does not trigger another GET")
			var tools mcp.ListToolsResult
			require.NoError(t, json.Unmarshal(legacyRPC(t, s, "tools/list", map[string]any{}), &tools))
			byName := map[string]mcp.Tool{}
			for _, tool := range tools.Tools {
				byName[tool.Name] = tool
			}
			for _, name := range []string{"signoz_list_alert_rules", "signoz_create_alert", "signoz_update_alert", "signoz_delete_alert", "signoz_list_notification_channels"} {
				require.Contains(t, byName, name)
			}
			require.Equal(t, []string{"uuid"}, byName["signoz_get_dashboard"].InputSchema.Required)
			require.Equal(t, []string{"query"}, byName["signoz_execute_builder_query"].InputSchema.Required)
		})
	}
}
