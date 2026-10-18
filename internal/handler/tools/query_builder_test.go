package tools

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	signozclient "github.com/SigNoz/signoz-mcp-server/internal/client"
	logpkg "github.com/SigNoz/signoz-mcp-server/pkg/log"
)

// SigNoz exhausts its memory on a raw query whose start is missing or 0
// (SigNoz/nerve-pod#439), so the guard has to stop the request before it leaves.
func TestExecuteBuilderQuery_RejectsMissingStartBeforeCallingSigNoz(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { upstreamCalls.Add(1) }))
	defer upstream.Close()
	h := newTestHandler(signozclient.NewClient(logpkg.New("error"), upstream.URL, "test-api-key", "SIGNOZ-API-KEY", nil))

	for name, arguments := range map[string]string{
		"missing start": `{"query":{"end":1700003600000,"requestType":"raw","compositeQuery":{"queries":[]}}}`,
		"zero start":    `{"query":{"start":0,"end":1700003600000,"requestType":"raw","compositeQuery":{"queries":[]}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			var args map[string]any
			require.NoError(t, json.Unmarshal([]byte(arguments), &args))
			result, err := h.handleExecuteBuilderQuery(testCtx(), makeToolRequest("signoz_execute_builder_query", args))
			require.NoError(t, err)
			require.Equal(t, CodeValidationFailed, resultCode(t, result))
			require.Contains(t, textContent(t, result), `"query.start" is required`)
		})
	}
	require.Zero(t, upstreamCalls.Load(), "the guard must stop the request before it reaches SigNoz")
}

// Exercise registration and MCP decoding too: the old typed handler discarded
// SQL/groupBy before the real HTTP client could send them.
func TestExecuteBuilderQuery_PreservesRequestAndUpstreamRejection(t *testing.T) {
	for name, query := range map[string]string{
		"SQL":                   `{"schemaVersion":"v1","start":1700000000000,"end":1700003600000,"requestType":"raw","compositeQuery":{"queries":[{"type":"clickhouse_sql","spec":{"name":"A","query":"SELECT name FROM system.tables WHERE database = 'offline_fixture' LIMIT 2","disabled":false,"legend":"tables"}}]}}`,
		"grouped signals":       `{"schemaVersion":"v1","start":1700000000000,"end":1700003600000,"requestType":"scalar","compositeQuery":{"queries":[{"type":"builder_query","spec":{"name":"A","signal":"logs","groupBy":[{"name":"service.name","signal":"logs","fieldContext":"resource"}],"aggregations":[{"expression":"count()"}],"limit":2,"order":[{"key":{"name":"count()"},"direction":"desc"}]}},{"type":"builder_query","spec":{"name":"B","signal":"metrics","groupBy":[{"name":"service.name","signal":"metrics","fieldContext":"resource"}],"aggregations":[{"metricName":"offline_fixture","spaceAggregation":"sum"}],"limit":2,"order":[{"key":{"name":"0"},"direction":"desc"}]}}]}}`,
		"unsupported SQL field": `{"schemaVersion":"v1","start":1700000000000,"end":1700003600000,"requestType":"raw","compositeQuery":{"queries":[{"type":"clickhouse_sql","spec":{"name":"A","query":"SELECT 1","signal":"unsupported"}}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			type received struct {
				method, path, auth string
				body               []byte
				err                error
			}
			requests := make(chan received, 8)
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				calls.Add(1)
				requests <- received{r.Method, r.URL.Path, r.Header.Get("SIGNOZ-API-KEY"), body, err}
				// Deliberately never execute SQL or manufacture query results.
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"status":"error","errorType":"invalid_input","error":"offline receiver rejected request; no query executed"}`)
			}))
			defer upstream.Close()

			h := newTestHandler(signozclient.NewClient(logpkg.New("error"), upstream.URL, "offline-fixture-only", "SIGNOZ-API-KEY", nil))
			server := newMCPTestServer()
			h.RegisterQueryBuilderV5Handlers(server)
			result := callRegisteredTool(t, server, "signoz_execute_builder_query", `{"query":`+query+`}`)

			require.Equal(t, int32(1), calls.Load(), "a rejected request must not be retried")
			got := <-requests
			require.NoError(t, got.err)
			require.Equal(t, http.MethodPost, got.method)
			require.Equal(t, "/api/v5/query_range", got.path)
			require.Equal(t, "offline-fixture-only", got.auth)
			require.JSONEq(t, query, string(got.body), "MCP-to-HTTP serialization must preserve query types and type-specific fields")
			require.True(t, result.IsError, "upstream rejection must never become a successful query result")
			require.Contains(t, textContent(t, result), "offline receiver rejected request; no query executed")
		})
	}
}
