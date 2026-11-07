package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOpenAPIContract(t *testing.T) {
	setTestAPIEnvironment(t)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	// An empty pool does not connect until acquired. The documentation must be
	// available without authentication or a database query.
	pool, err := pgxpool.New(t.Context(), "postgres://scopeguard@127.0.0.1:1/scopeguard?sslmode=disable&pool_max_conns=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	router, err := NewRouter(pool, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("expected public OpenAPI document, got HTTP %d: %s", response.Code, response.Body.String())
	}
	if !strings.HasPrefix(response.Header().Get("Content-Type"), "application/json") {
		t.Fatal("OpenAPI URL must serve JSON")
	}

	var document struct {
		OpenAPI string `json:"openapi"`
		Paths   map[string]map[string]struct {
			OperationID string                     `json:"operationId"`
			Responses   map[string]json.RawMessage `json:"responses"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatalf("invalid OpenAPI JSON: %v", err)
	}
	if !strings.HasPrefix(document.OpenAPI, "3.") || len(document.Paths) == 0 {
		t.Fatal("expected an OpenAPI 3 document with endpoint definitions")
	}

	t.Run("routes", func(t *testing.T) {
		registered := make(map[string]bool)
		for _, route := range router.Routes() {
			if route.Path == "/openapi.json" {
				continue
			}
			segments := strings.Split(route.Path, "/")
			for i, segment := range segments {
				if strings.HasPrefix(segment, ":") {
					segments[i] = "{" + strings.TrimPrefix(segment, ":") + "}"
				}
			}
			registered[route.Method+" "+strings.Join(segments, "/")] = true
		}

		operationIDs := make(map[string]bool)
		for path, methods := range document.Paths {
			for method, operation := range methods {
				key := strings.ToUpper(method) + " " + path
				if !registered[key] {
					t.Errorf("OpenAPI documents an unregistered endpoint: %s", key)
				}
				delete(registered, key)
				if operation.OperationID == "" || operationIDs[operation.OperationID] {
					t.Errorf("%s must have a unique, nonempty operationId", key)
				}
				operationIDs[operation.OperationID] = true
				if len(operation.Responses) == 0 {
					t.Errorf("%s must document its responses", key)
				}
			}
		}
		for key := range registered {
			t.Errorf("endpoint is missing from OpenAPI: %s", key)
		}
	})

	t.Run("references", func(t *testing.T) {
		var root any
		if err := json.Unmarshal(response.Body.Bytes(), &root); err != nil {
			t.Fatal(err)
		}
		var checkReferences func(any)
		checkReferences = func(value any) {
			switch value := value.(type) {
			case map[string]any:
				if ref, ok := value["$ref"].(string); ok {
					if !strings.HasPrefix(ref, "#/") {
						t.Errorf("OpenAPI must be self-contained for file import: %s", ref)
						return
					}
					target := root
					for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
						part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
						object, ok := target.(map[string]any)
						if !ok || object[part] == nil {
							t.Errorf("unresolved OpenAPI reference: %s", ref)
							break
						}
						target = object[part]
					}
				}
				for _, child := range value {
					checkReferences(child)
				}
			case []any:
				for _, child := range value {
					checkReferences(child)
				}
			}
		}
		checkReferences(root)
	})
}
