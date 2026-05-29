package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nimling/nimo-api/internal"
)

const inlineSpecPath = "../examples/inline_spec.yml"

func runInlineConvert(t *testing.T, outputDir string, opts internal.InlineOptions) map[string]any {
	t.Helper()

	if err := internal.RunConvert(
		[]string{inlineSpecPath},
		"",
		outputDir,
		"",
		false,
		"",
		"",
		"",
		false,
		false,
		opts,
	); err != nil {
		t.Fatalf("RunConvert failed: %v", err)
	}

	var specPath string
	walkErr := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Base(path) == "spec.json" {
			specPath = path
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("failed to locate produced spec: %v", walkErr)
	}
	if specPath == "" {
		t.Fatalf("no spec.json produced under %s", outputDir)
	}
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("failed to read produced spec %s: %v", specPath, err)
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("failed to parse produced spec: %v", err)
	}

	if strings.Contains(string(data), "$REF_NOT_RESOLVED") {
		t.Fatalf("spec contains unresolved ref markers")
	}

	return doc
}

func getMap(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("expected %q to be an object, got %T", key, parent[key])
	}
	return value
}

func getOp(t *testing.T, doc map[string]any, path, verb string) map[string]any {
	t.Helper()
	paths := getMap(t, doc, "paths")
	pathItem := getMap(t, paths, path)
	return getMap(t, pathItem, verb)
}

func TestInlineExamplesRemovesComponentRefs(t *testing.T) {
	dir := t.TempDir()
	doc := runInlineConvert(t, dir, internal.InlineOptions{Examples: true})

	op := getOp(t, doc, "/things", "get")
	responses := getMap(t, op, "responses")
	ok := getMap(t, responses, "200")
	content := getMap(t, ok, "content")
	json := getMap(t, content, "application/json")
	examples := getMap(t, json, "examples")
	def := getMap(t, examples, "default")

	if _, present := def["$ref"]; present {
		t.Fatalf("example still has $ref after InlineExamples: %v", def)
	}
	value, ok2 := def["value"].(map[string]any)
	if !ok2 {
		t.Fatalf("example value missing or wrong type, got %T", def["value"])
	}
	if value["id"] != "t_1" {
		t.Fatalf("unexpected example id: %v", value["id"])
	}

	if components, hasComponents := doc["components"].(map[string]any); hasComponents {
		if _, stillThere := components["examples"]; stillThere {
			t.Fatalf("components.examples should be removed after InlineExamples")
		}
	}
}

func TestInlineResponsesRemovesComponentRefs(t *testing.T) {
	dir := t.TempDir()
	doc := runInlineConvert(t, dir, internal.InlineOptions{Responses: true})

	op := getOp(t, doc, "/things", "get")
	responses := getMap(t, op, "responses")
	badRequest := getMap(t, responses, "400")
	if _, hasRef := badRequest["$ref"]; hasRef {
		t.Fatalf("response 400 still has $ref after InlineResponses: %v", badRequest)
	}
	if desc, _ := badRequest["description"].(string); desc != "Malformed request" {
		t.Fatalf("inlined 400 description wrong: %v", badRequest["description"])
	}
	if _, hasContent := badRequest["content"].(map[string]any); !hasContent {
		t.Fatalf("inlined 400 missing content block")
	}

	if components, hasComponents := doc["components"].(map[string]any); hasComponents {
		if _, stillThere := components["responses"]; stillThere {
			t.Fatalf("components.responses should be removed after InlineResponses")
		}
	}
}

func TestInlineSchemasRemovesComponentRefs(t *testing.T) {
	dir := t.TempDir()
	doc := runInlineConvert(t, dir, internal.InlineOptions{Schemas: true})

	op := getOp(t, doc, "/things", "get")
	responses := getMap(t, op, "responses")
	ok := getMap(t, responses, "200")
	content := getMap(t, ok, "content")
	json := getMap(t, content, "application/json")
	schema := getMap(t, json, "schema")
	if _, hasRef := schema["$ref"]; hasRef {
		t.Fatalf("schema still has $ref after InlineSchemas: %v", schema)
	}
	if typ, _ := schema["type"].(string); typ != "object" {
		t.Fatalf("inlined schema type wrong: %v", schema["type"])
	}
	props, ok2 := schema["properties"].(map[string]any)
	if !ok2 {
		t.Fatalf("inlined schema missing properties")
	}
	if _, hasName := props["name"]; !hasName {
		t.Fatalf("inlined schema missing name property")
	}

	if components, hasComponents := doc["components"].(map[string]any); hasComponents {
		if _, stillThere := components["schemas"]; stillThere {
			t.Fatalf("components.schemas should be removed after InlineSchemas")
		}
	}
}

func TestFlatRemovesAllComponentRefs(t *testing.T) {
	dir := t.TempDir()
	doc := runInlineConvert(t, dir, internal.InlineOptions{Examples: true, Responses: true, Schemas: true})

	op := getOp(t, doc, "/things", "get")
	responses := getMap(t, op, "responses")
	ok := getMap(t, responses, "200")
	content := getMap(t, ok, "content")
	json := getMap(t, content, "application/json")
	schema := getMap(t, json, "schema")
	if _, hasRef := schema["$ref"]; hasRef {
		t.Fatalf("schema still has $ref after flat inline")
	}

	examples := getMap(t, json, "examples")
	def := getMap(t, examples, "default")
	if _, hasRef := def["$ref"]; hasRef {
		t.Fatalf("example still has $ref after flat inline")
	}

	badRequest := getMap(t, responses, "400")
	if _, hasRef := badRequest["$ref"]; hasRef {
		t.Fatalf("response 400 still has $ref after flat inline")
	}

	if components, hasComponents := doc["components"].(map[string]any); hasComponents {
		for _, key := range []string{"examples", "responses", "schemas"} {
			if _, stillThere := components[key]; stillThere {
				t.Fatalf("components.%s should be removed after flat inline", key)
			}
		}
	}
}

func TestNoInlineLeavesRefs(t *testing.T) {
	dir := t.TempDir()
	doc := runInlineConvert(t, dir, internal.InlineOptions{})

	op := getOp(t, doc, "/things", "get")
	responses := getMap(t, op, "responses")
	badRequest := getMap(t, responses, "400")
	if _, hasRef := badRequest["$ref"]; !hasRef {
		t.Fatalf("response 400 should keep $ref when no inline flag is set: %v", badRequest)
	}

	ok := getMap(t, responses, "200")
	content := getMap(t, ok, "content")
	json := getMap(t, content, "application/json")
	examples := getMap(t, json, "examples")
	def := getMap(t, examples, "default")
	if _, hasRef := def["$ref"]; !hasRef {
		t.Fatalf("example default should keep $ref when no inline flag is set: %v", def)
	}
}
