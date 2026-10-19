package tools

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
)

// Both http.DefaultClient and newly constructed clients use the same private Unix
// backend; production methods/auth/serialization run normally, without a shared port.
func TestDashboardsV2Consumer(t *testing.T) {
	consumer := func(t *testing.T, receiver http.HandlerFunc) *server.MCPServer {
		s := legacyConsumer(t, receiver)
		previous := http.DefaultTransport
		http.DefaultTransport = http.DefaultClient.Transport
		t.Cleanup(func() { http.DefaultTransport = previous })
		return s
	}
	t.Run("catalog", func(t *testing.T) {
		s := consumer(t, func(w http.ResponseWriter, r *http.Request) { t.Error("catalog must not call backend") })
		var catalog struct {
			Tools []struct {
				Name        string         `json:"name"`
				InputSchema map[string]any `json:"inputSchema"`
				Description string         `json:"description"`
			} `json:"tools"`
		}
		require.NoError(t, json.Unmarshal(legacyRPC(t, s, "tools/list", map[string]any{}), &catalog))
		found := 0
		for _, tool := range catalog.Tools {
			if tool.Name == "signoz_create_dashboard" {
				props := tool.InputSchema["properties"].(map[string]any)
				require.Contains(t, props, "spec")
				require.Contains(t, props, "schemaVersion")
				require.NotContains(t, props, "widgets")
				require.NotContains(t, props, "title")
				require.Contains(t, tool.Description, "v2")
				found++
			}
			if tool.Name == "signoz_update_dashboard" {
				require.Contains(t, tool.InputSchema["properties"].(map[string]any), "uuid")
				found++
			}
		}
		require.Equal(t, 2, found)
	})
	t.Run("server_pages_and_summaries", func(t *testing.T) {
		var calls atomic.Int32
		s := consumer(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/api/v2/dashboards", r.URL.Path)
			require.Equal(t, "50", r.URL.Query().Get("limit"))
			require.Equal(t, "request-fixture-key", r.Header.Get("SIGNOZ-API-KEY"))
			offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
			require.NoError(t, err)
			require.Contains(t, []int{0, 1}, offset)
			// Simulate the API capping a requested page: advance by returned rows, not 50.
			_, _ = io.WriteString(w, `{"status":"success","data":{"dashboards":[{"id":"id-`+strconv.Itoa(offset)+`","spec":{"display":{"name":"Visible dashboard","description":"Offline fixture"}},"tags":[{"key":"env","value":"test"}],"createdAt":"2026-01-01"}],"total":2}}`)
		})
		for _, offset := range []string{"0", "1"} {
			result := legacyCall(t, s, "signoz_list_dashboards", map[string]any{"limit": "50", "offset": offset})
			require.False(t, result.IsError, legacyText(t, result))
			var page struct {
				Data       []map[string]any `json:"data"`
				Pagination struct {
					Total      int  `json:"total"`
					HasMore    bool `json:"hasMore"`
					NextOffset int  `json:"nextOffset"`
				} `json:"pagination"`
			}
			require.NoError(t, json.Unmarshal([]byte(legacyText(t, result)), &page))
			require.Len(t, page.Data, 1)
			require.Equal(t, "Visible dashboard", page.Data[0]["name"])
			require.Equal(t, "id-"+offset, page.Data[0]["uuid"])
			require.Equal(t, 2, page.Pagination.Total)
			if offset == "0" {
				require.True(t, page.Pagination.HasMore)
				require.Equal(t, 1, page.Pagination.NextOffset)
			} else {
				require.False(t, page.Pagination.HasMore)
				require.Equal(t, -1, page.Pagination.NextOffset)
			}
		}
		require.Equal(t, int32(2), calls.Load())
	})
	t.Run("empty_page", func(t *testing.T) {
		s := consumer(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"status":"success","data":{"dashboards":[],"total":0}}`)
		})
		result := legacyCall(t, s, "signoz_list_dashboards", map[string]any{})
		require.False(t, result.IsError)
		require.Contains(t, legacyText(t, result), `"hasMore":false`)
	})
	for name, body := range map[string]string{"old_shape": `{"data":[]}`, "missing_total": `{"data":{"dashboards":[]}}`, "empty_before_total": `{"data":{"dashboards":[],"total":4}}`} {
		t.Run("reject_list_"+name, func(t *testing.T) {
			s := consumer(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, body) })
			require.True(t, legacyCall(t, s, "signoz_list_dashboards", map[string]any{}).IsError)
		})
	}
	t.Run("mixed_legacy_page", func(t *testing.T) {
		var calls atomic.Int32
		s := consumer(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/api/v2/dashboards", r.URL.Path)
			_, _ = io.WriteString(w, `{"status":"success","data":{"dashboards":[{"id":"normal-id","name":"normal-slug","spec":{"display":{"name":"Normal dashboard"}}},{"id":"legacy-id","name":"legacy-slug","legacy":true,"spec":{"display":{"name":""}}}],"total":3}}`)
		})
		result := legacyCall(t, s, "signoz_list_dashboards", map[string]any{})
		require.False(t, result.IsError, legacyText(t, result))
		var page struct {
			Data       []map[string]any `json:"data"`
			Pagination struct {
				Total      int  `json:"total"`
				HasMore    bool `json:"hasMore"`
				NextOffset int  `json:"nextOffset"`
			} `json:"pagination"`
		}
		require.NoError(t, json.Unmarshal([]byte(legacyText(t, result)), &page))
		require.Len(t, page.Data, 2)
		require.Equal(t, "normal-id", page.Data[0]["uuid"])
		require.Equal(t, "Normal dashboard", page.Data[0]["name"])
		require.Equal(t, "legacy-id", page.Data[1]["uuid"])
		require.Equal(t, "legacy-slug", page.Data[1]["name"])
		require.Equal(t, 3, page.Pagination.Total)
		require.True(t, page.Pagination.HasMore)
		require.Equal(t, 2, page.Pagination.NextOffset)
		require.Equal(t, int32(1), calls.Load())
	})
	const payload = `{"name":"offline-dashboard","schemaVersion":"v6","image":"/assets/Icons/eight-ball","tags":[{"key":"env","value":"test"}],"spec":{"display":{"name":"Offline dashboard","description":"Preserve full state"},"layouts":[],"panels":{"text":{"kind":"Panel","spec":{"plugin":{"kind":"TextPanel","spec":{"text":"unchanged"}},"queries":[{"kind":"raw","spec":{"plugin":{"kind":"signoz/CompositeQuery","spec":{"queries":[{"type":"clickhouse_sql","spec":{"name":"A","query":"SELECT 1","disabled":false,"legend":""}}]}}}}],"threshold":0}}},"variables":[],"links":null,"datasources":null}}`
	t.Run("explicit_tags_for_full_write", func(t *testing.T) {
		var calls atomic.Int32
		s := consumer(t, func(w http.ResponseWriter, r *http.Request) {
			n := calls.Add(1)
			expected := http.MethodPost
			if n == 2 {
				expected = http.MethodPut
			}
			require.Equal(t, expected, r.Method)
			require.Equal(t, "request-fixture-key", r.Header.Get("SIGNOZ-API-KEY"))
			var body map[string]json.RawMessage
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.JSONEq(t, `[]`, string(body["tags"]), "an intentional empty array must remain an array")
			_, _ = io.WriteString(w, `{"status":"success","data":{"id":"offline-only"}}`)
		})
		args := func(mode, tagCase string) map[string]any {
			var dashboard map[string]any
			require.NoError(t, json.Unmarshal([]byte(payload), &dashboard))
			switch tagCase {
			case "absent":
				delete(dashboard, "tags")
			case "null":
				dashboard["tags"] = nil
			case "empty":
				dashboard["tags"] = []any{}
			}
			if mode == "update" {
				return map[string]any{"uuid": "existing", "dashboard": dashboard}
			}
			return dashboard
		}
		for _, mode := range []string{"create", "update"} {
			for _, tagCase := range []string{"absent", "null"} {
				result := legacyCall(t, s, "signoz_"+mode+"_dashboard", args(mode, tagCase))
				require.True(t, result.IsError)
				require.Contains(t, legacyText(t, result), "tags must be an explicit array")
			}
		}
		require.Zero(t, calls.Load(), "absent/null tags must not reach an HTTP write")
		for _, mode := range []string{"create", "update"} {
			result := legacyCall(t, s, "signoz_"+mode+"_dashboard", args(mode, "empty"))
			require.False(t, result.IsError, legacyText(t, result))
		}
		require.Equal(t, int32(2), calls.Load(), "intentional empty tags must reach both v2 write paths")
	})

	for _, mode := range []string{"create", "update"} {
		successStatus := http.StatusCreated
		if mode == "update" {
			successStatus = http.StatusOK
		}
		for _, status := range []int{successStatus, http.StatusBadRequest} {
			t.Run(mode+"_"+strconv.Itoa(status), func(t *testing.T) {
				var calls atomic.Int32
				s := consumer(t, func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					method, path := http.MethodPost, "/api/v2/dashboards"
					if mode == "update" {
						method = http.MethodPut
						path += "/offline%2Fdashboard%3Fx%23y"
					}
					require.Equal(t, method, r.Method)
					require.Equal(t, path, r.URL.EscapedPath())
					require.Equal(t, "request-fixture-key", r.Header.Get("SIGNOZ-API-KEY"))
					require.Equal(t, "application/json", r.Header.Get("Content-Type"))
					got, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					require.JSONEq(t, payload, string(got))
					w.WriteHeader(status)
					if status == http.StatusBadRequest {
						_, _ = io.WriteString(w, `{"error":{"code":"offline_dashboard_invalid","message":"backend rejected fixture"}}`)
					} else {
						_, _ = io.WriteString(w, `{"status":"success","data":{"id":"created-offline"}}`)
					}
				})
				var args map[string]any
				require.NoError(t, json.Unmarshal([]byte(payload), &args))
				if mode == "update" {
					args = map[string]any{"uuid": "offline/dashboard?x#y", "dashboard": args}
				}
				result := legacyCall(t, s, "signoz_"+mode+"_dashboard", args)
				require.Equal(t, int32(1), calls.Load())
				if status == http.StatusBadRequest {
					require.True(t, result.IsError)
					require.Contains(t, legacyText(t, result), "offline_dashboard_invalid")
				} else {
					require.False(t, result.IsError, legacyText(t, result))
					if mode == "create" {
						require.Contains(t, legacyText(t, result), "created-offline")
					} else {
						require.Equal(t, "dashboard updated", legacyText(t, result))
					}
				}
			})
		}
	}
	t.Run("reject_legacy_or_partial_before_write", func(t *testing.T) {
		var calls atomic.Int32
		s := consumer(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(http.StatusTeapot) })
		for _, input := range []string{`{"title":"old","layout":[],"widgets":[]}`, `{"name":"partial","schemaVersion":"v6","spec":{"display":{"name":"partial"}}}`, `{"name":"old-version","schemaVersion":"v5","spec":{}}`, `{"name":"missing-spec","schemaVersion":"v6","spec":null}`} {
			var args map[string]any
			require.NoError(t, json.Unmarshal([]byte(input), &args))
			require.True(t, legacyCall(t, s, "signoz_create_dashboard", args).IsError)
			require.True(t, legacyCall(t, s, "signoz_update_dashboard", map[string]any{"uuid": "existing", "dashboard": args}).IsError)
		}
		require.Zero(t, calls.Load(), "unsupported input must not reach an HTTP mutation")
	})
}
