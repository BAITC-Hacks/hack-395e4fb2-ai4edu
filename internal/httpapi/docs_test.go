package httpapi

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func apiDocument(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../docs/api.md")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestAPIDocumentedValidatorCodes(t *testing.T) {
	doc := apiDocument(t)
	file, err := parser.ParseFile(token.NewFileSet(), "../simulation/validator.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	codes := make(map[string]bool)
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name, ok := call.Fun.(*ast.Ident)
		if !ok || name.Name != "add" {
			return true
		}
		if len(call.Args) == 0 {
			t.Fatal("validator add call has no code; update documentation coverage test")
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			t.Fatal("validator code is no longer a string literal; update documentation coverage test")
		}
		code, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Fatal(err)
		}
		codes[code] = true
		if !strings.Contains(doc, "| `"+code+"` | 422 |") {
			t.Errorf("validator code %q has no entry in docs/api.md error table", code)
		}
		return true
	})
	if len(codes) == 0 {
		t.Fatal("no validator codes found; update documentation coverage test")
	}
}

// Markers keep examples machine-readable without relying on translated headings.
func documentedJSON(t *testing.T, doc, name string) string {
	t.Helper()
	marker := "<!-- example:" + name + " -->\n```json\n"
	if strings.Count(doc, marker) != 1 {
		t.Fatalf("expected exactly one example marker %q", name)
	}
	_, rest, _ := strings.Cut(doc, marker)
	body, _, ok := strings.Cut(rest, "\n```")
	if !ok {
		t.Fatalf("unclosed JSON example %q", name)
	}
	return body
}

func jsonObject(t *testing.T, data string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(data), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestAPIExamples(t *testing.T) {
	doc := apiDocument(t)
	handler := NewHandler()
	for _, tc := range []struct {
		name   string
		status int
	}{
		{"golden", 200},
		{"invalid", 422},
		{"malformed", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := documentedJSON(t, doc, tc.name+"-request")
			if tc.name == "golden" && !reflect.DeepEqual(jsonObject(t, request), jsonObject(t, goldenJSON)) {
				t.Fatal("documented golden request differs from regression scenario")
			}
			response := call(handler, "POST", "/api/simulate", request)
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, tc.status, response.Body.String())
			}
			want := jsonObject(t, documentedJSON(t, doc, tc.name+"-response"))
			got := jsonObject(t, response.Body.String())
			if tc.name == "golden" {
				for _, field := range []string{"total_cost", "final_score", "critical_after"} {
					value, present := want[field]
					if !present || value != got[field] {
						t.Errorf("documented %s = %v, server = %v", field, value, got[field])
					}
				}
			}
			// Check the complete response too, allowing only JSON formatting changes.
			if !reflect.DeepEqual(want, got) {
				t.Errorf("docs/api.md %s response is stale; capture the actual server response again", tc.name)
			}
		})
	}
}
