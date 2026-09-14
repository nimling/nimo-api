package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nimling/nimo-api/internal"
)

// Specification extensions (x- keys) on operations and schemas must survive
// conversion; a closed struct used to drop them silently at unmarshal.
func TestConvertPreservesExtensions(t *testing.T) {
	dir := t.TempDir()
	spec := `openapi: 3.1.0
info:
  title: Extensions API
  version: 1.0.0
  description: Extension passthrough test
servers:
  - url: https://api.example.com
    description: Test server
paths:
  /things:
    get:
      summary: List things
      description: Lists things
      operationId: listThings
      x-response-variants:
        param: compact
        cases:
          "true": ThingCompact
        default: Thing
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Thing'
components:
  schemas:
    Thing:
      type: object
      x-entity:
        key: [id]
      properties:
        id:
          type: string
    ThingCompact:
      type: object
      properties:
        id:
          type: string
`
	specPath := filepath.Join(dir, "spec.yml")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "out")
	cmd := internal.NewConvertCommand()
	cmd.SetArgs([]string{specPath, "-o", out, "--inline-examples", "--inline-responses", "--inline-parameters"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	var specPath2 string
	_ = filepath.WalkDir(out, func(path string, d os.DirEntry, err error) error {
		if err == nil && d.Name() == "spec.json" {
			specPath2 = path
		}
		return nil
	})
	if specPath2 == "" {
		t.Fatal("spec.json not written")
	}
	raw, err := os.ReadFile(specPath2)
	if err != nil {
		t.Fatalf("spec.json not readable: %v", err)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("spec.json is not valid json: %v", err)
	}

	text := string(raw)
	if !strings.Contains(text, "x-response-variants") {
		t.Error("operation extension x-response-variants was dropped")
	}
	if !strings.Contains(text, "x-entity") {
		t.Error("schema extension x-entity was dropped")
	}
}
