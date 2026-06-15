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
		false,
		"",
		"",
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

	resolveSplitOperations(t, doc, filepath.Dir(specPath))

	return doc
}

func resolveSplitOperations(t *testing.T, doc map[string]any, specDir string) {
	t.Helper()
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		return
	}
	for _, pathItemRaw := range paths {
		pathItem, ok := pathItemRaw.(map[string]any)
		if !ok {
			continue
		}
		for verb, opRaw := range pathItem {
			op, ok := opRaw.(map[string]any)
			if !ok {
				continue
			}
			ref, ok := op["$ref"].(string)
			if !ok || !strings.HasPrefix(ref, "./") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(specDir, ref))
			if err != nil {
				t.Fatalf("failed to read split operation %s: %v", ref, err)
			}
			var loaded map[string]any
			if err := json.Unmarshal(data, &loaded); err != nil {
				t.Fatalf("failed to parse split operation %s: %v", ref, err)
			}
			pathItem[verb] = loaded
		}
	}
}

func getMap(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("expected %q to be an object, got %T", key, parent[key])
	}
	return value
}

func getSlice(t *testing.T, parent map[string]any, key string) []any {
	t.Helper()
	value, ok := parent[key].([]any)
	if !ok {
		t.Fatalf("expected %q to be an array, got %T", key, parent[key])
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

func TestInlineParametersRemovesComponentRefs(t *testing.T) {
	dir := t.TempDir()
	doc := runInlineConvert(t, dir, internal.InlineOptions{Parameters: true})

	op := getOp(t, doc, "/things", "get")
	params := getSlice(t, op, "parameters")
	if len(params) == 0 {
		t.Fatalf("expected at least one parameter on GET /things")
	}
	first, ok := params[0].(map[string]any)
	if !ok {
		t.Fatalf("expected parameter to be an object, got %T", params[0])
	}
	if _, hasRef := first["$ref"]; hasRef {
		t.Fatalf("parameter still has $ref after InlineParameters: %v", first)
	}
	if first["name"] != "limit" {
		t.Fatalf("inlined parameter name wrong: %v", first["name"])
	}
	if first["in"] != "query" {
		t.Fatalf("inlined parameter in wrong: %v", first["in"])
	}

	if components, hasComponents := doc["components"].(map[string]any); hasComponents {
		if _, stillThere := components["parameters"]; stillThere {
			t.Fatalf("components.parameters should be removed after InlineParameters")
		}
	}
}

func TestInlineSchemasDedupesToComponentRefs(t *testing.T) {
	dir := t.TempDir()
	doc := runInlineConvert(t, dir, internal.InlineOptions{Schemas: true})

	op := getOp(t, doc, "/things", "get")
	responses := getMap(t, op, "responses")
	ok := getMap(t, responses, "200")
	content := getMap(t, ok, "content")
	json := getMap(t, content, "application/json")
	schema := getMap(t, json, "schema")
	ref, hasRef := schema["$ref"].(string)
	if !hasRef {
		t.Fatalf("schema should keep a component $ref after InlineSchemas dedupe: %v", schema)
	}
	if ref != "#/components/schemas/Thing" {
		t.Fatalf("schema ref points at wrong target: %v", ref)
	}

	components := getMap(t, doc, "components")
	schemas := getMap(t, components, "schemas")
	if _, hasThing := schemas["Thing"]; !hasThing {
		t.Fatalf("components.schemas.Thing should remain after InlineSchemas dedupe")
	}
}

func TestFlatInlinesEverythingButSchemaDedupes(t *testing.T) {
	dir := t.TempDir()
	doc := runInlineConvert(t, dir, internal.InlineOptions{Examples: true, Responses: true, Schemas: true, Parameters: true})

	op := getOp(t, doc, "/things", "get")

	params := getSlice(t, op, "parameters")
	first, _ := params[0].(map[string]any)
	if _, hasRef := first["$ref"]; hasRef {
		t.Fatalf("parameter still has $ref after flat inline")
	}

	responses := getMap(t, op, "responses")
	ok := getMap(t, responses, "200")
	content := getMap(t, ok, "content")
	json := getMap(t, content, "application/json")

	schema := getMap(t, json, "schema")
	if _, hasRef := schema["$ref"]; !hasRef {
		t.Fatalf("schema should keep a component $ref after flat inline dedupe")
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

	components := getMap(t, doc, "components")
	for _, key := range []string{"examples", "responses", "parameters"} {
		if _, stillThere := components[key]; stillThere {
			t.Fatalf("components.%s should be removed after flat inline", key)
		}
	}
	if _, hasSchemas := components["schemas"]; !hasSchemas {
		t.Fatalf("components.schemas should remain after flat inline dedupe")
	}
}

func TestFlatWritesSingleFile(t *testing.T) {
	dir := t.TempDir()
	doc := runInlineConvert(t, dir, internal.InlineOptions{Examples: true, Responses: true, Schemas: true, Parameters: true})

	if hasSubdir(t, dir, "operations") {
		t.Fatalf("flat inline must not write an operations directory")
	}
	if hasSubdir(t, dir, "schemas") {
		t.Fatalf("flat inline must not write a schemas directory")
	}

	paths := getMap(t, doc, "paths")
	pathItem := getMap(t, paths, "/things")
	op := getMap(t, pathItem, "get")
	if _, isRef := op["$ref"]; isRef {
		t.Fatalf("operation must be inline in a single-file spec, found a $ref")
	}
}

func TestSplitWritesOperationFiles(t *testing.T) {
	dir := t.TempDir()
	runInlineConvert(t, dir, internal.InlineOptions{})

	if !hasSubdir(t, dir, "operations") {
		t.Fatalf("non-inline convert should split into an operations directory")
	}
}

func hasSubdir(t *testing.T, root, name string) bool {
	t.Helper()
	found := false
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == name {
			found = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s failed: %v", root, err)
	}
	return found
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

func TestNormalizeRewritesAdditionalPropertiesFileRef(t *testing.T) {
	dir := t.TempDir()
	doc := runInlineConvert(t, dir, internal.InlineOptions{Schemas: true})

	components := getMap(t, doc, "components")
	schemas := getMap(t, components, "schemas")
	thingMap := getMap(t, schemas, "ThingMap")
	additional := getMap(t, thingMap, "additionalProperties")
	ref, ok := additional["$ref"].(string)
	if !ok {
		t.Fatalf("ThingMap.additionalProperties should carry a $ref, got %v", additional)
	}
	if ref != "#/components/schemas/Thing" {
		t.Fatalf("additionalProperties file ref should normalize to an internal pointer, got %q", ref)
	}
}
