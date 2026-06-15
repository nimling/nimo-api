package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nimling/nimo-api/converter"
)

const bookableSpecRelPath = "../../../samna/bookable_server/openapi_definition/index.yml"

func skipIfNoBookableSpec(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(bookableSpecRelPath)
	if err != nil {
		t.Fatalf("failed to resolve bookable spec path: %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Skipf("bookable spec not available at %s: %v", abs, err)
	}
	return abs
}

func convertBookableSpec(t *testing.T) (*converter.OpenAPIConverter, string) {
	t.Helper()
	specPath := skipIfNoBookableSpec(t)

	conv, err := converter.NewOpenApiConverter(specPath)
	if err != nil {
		t.Fatalf("failed to load bookable spec: %v", err)
	}
	if err := conv.ValidateDocument(); err != nil {
		t.Fatalf("validation error: %v", err)
	}

	outDir := t.TempDir()
	if err := conv.WriteOpenAPISpec(outDir); err != nil {
		t.Fatalf("failed to write joint spec: %v", err)
	}

	specOut := filepath.Join(outDir, conv.CommonPrefix, "spec.json")
	if _, err := os.Stat(specOut); err != nil {
		t.Fatalf("joint spec file missing at %s: %v", specOut, err)
	}
	return conv, specOut
}

func readJointSpec(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read joint spec: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("failed to parse joint spec as JSON: %v", err)
	}
	return doc
}

func TestBookableSpecConverts(t *testing.T) {
	_, specOut := convertBookableSpec(t)
	doc := readJointSpec(t, specOut)

	if _, ok := doc["paths"]; !ok {
		t.Fatalf("joint spec missing paths")
	}
	if _, ok := doc["components"]; !ok {
		t.Fatalf("joint spec missing components")
	}
}

func TestBookableJointSpecHasExamplesComponent(t *testing.T) {
	_, specOut := convertBookableSpec(t)
	doc := readJointSpec(t, specOut)

	components, ok := doc["components"].(map[string]any)
	if !ok {
		t.Fatalf("components not an object")
	}
	examples, ok := components["examples"].(map[string]any)
	if !ok {
		t.Fatalf("components.examples missing or wrong type")
	}
	if len(examples) == 0 {
		t.Fatalf("expected at least one resolved example under components.examples")
	}

	for name, raw := range examples {
		obj, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("components.examples.%s is not an object", name)
		}
		if _, hasRef := obj["$ref"]; hasRef {
			t.Fatalf("components.examples.%s still contains $ref after resolution", name)
		}
		if _, hasValue := obj["value"]; !hasValue {
			t.Fatalf("components.examples.%s missing value field", name)
		}
	}
}

func TestBookableGetMeRefRewritten(t *testing.T) {
	_, specOut := convertBookableSpec(t)
	doc := readJointSpec(t, specOut)

	resp := findResponseExamples(t, doc, "/me", "get", "200")
	entry, ok := resp["default"].(map[string]any)
	if !ok {
		t.Fatalf("/me 200 examples.default missing or wrong type")
	}
	ref, ok := entry["$ref"].(string)
	if !ok {
		t.Fatalf("/me 200 examples.default did not become a $ref entry: %v", entry)
	}
	if !strings.HasPrefix(ref, "#/components/examples/") {
		t.Fatalf("expected internal example ref, got %q", ref)
	}

	name := strings.TrimPrefix(ref, "#/components/examples/")
	components := doc["components"].(map[string]any)
	examples := components["examples"].(map[string]any)
	target, ok := examples[name].(map[string]any)
	if !ok {
		t.Fatalf("components.examples.%s missing", name)
	}
	value, ok := target["value"].(map[string]any)
	if !ok {
		t.Fatalf("components.examples.%s value missing or wrong type", name)
	}
	if _, ok := value["data"]; !ok {
		t.Fatalf("get_me example value missing data field")
	}
}

func TestBookablePostBookingExamplesResolved(t *testing.T) {
	_, specOut := convertBookableSpec(t)
	doc := readJointSpec(t, specOut)

	reqExamples := findRequestBodyExamples(t, doc, "/booking", "post")
	assertExampleResolvesToValue(t, doc, reqExamples, "default")

	created := findResponseExamples(t, doc, "/booking", "post", "201")
	assertExampleResolvesToValue(t, doc, created, "default")
}

func assertExampleResolvesToValue(t *testing.T, doc map[string]any, examples map[string]any, key string) {
	t.Helper()
	entry, ok := examples[key].(map[string]any)
	if !ok {
		t.Fatalf("example %q missing or wrong type", key)
	}
	ref, ok := entry["$ref"].(string)
	if !ok {
		t.Fatalf("example %q did not become an internal ref entry: %v", key, entry)
	}
	if !strings.HasPrefix(ref, "#/components/examples/") {
		t.Fatalf("example %q expected internal example ref, got %q", key, ref)
	}

	name := strings.TrimPrefix(ref, "#/components/examples/")
	components, ok := doc["components"].(map[string]any)
	if !ok {
		t.Fatalf("components missing")
	}
	comExamples, ok := components["examples"].(map[string]any)
	if !ok {
		t.Fatalf("components.examples missing")
	}
	target, ok := comExamples[name].(map[string]any)
	if !ok {
		t.Fatalf("components.examples.%s missing", name)
	}
	if _, hasValue := target["value"]; !hasValue {
		t.Fatalf("components.examples.%s missing value field", name)
	}
}

func TestBookableJointSpecInlinesExampleFiles(t *testing.T) {
	_, specOut := convertBookableSpec(t)
	raw, err := os.ReadFile(specOut)
	if err != nil {
		t.Fatalf("failed to read joint spec: %v", err)
	}
	if strings.Contains(string(raw), "../examples/") {
		t.Fatalf("joint spec still contains external example path")
	}
}

func findResponseExamples(t *testing.T, doc map[string]any, path, method, status string) map[string]any {
	t.Helper()
	op := findOperation(t, doc, path, method)
	responses, ok := op["responses"].(map[string]any)
	if !ok {
		t.Fatalf("%s %s responses missing", method, path)
	}
	resp, ok := responses[status].(map[string]any)
	if !ok {
		t.Fatalf("%s %s response %s missing", method, path, status)
	}
	content, ok := resp["content"].(map[string]any)
	if !ok {
		t.Fatalf("%s %s response %s content missing", method, path, status)
	}
	media, ok := content["application/json"].(map[string]any)
	if !ok {
		t.Fatalf("%s %s response %s application/json missing", method, path, status)
	}
	examples, ok := media["examples"].(map[string]any)
	if !ok {
		t.Fatalf("%s %s response %s examples missing", method, path, status)
	}
	return examples
}

func findRequestBodyExamples(t *testing.T, doc map[string]any, path, method string) map[string]any {
	t.Helper()
	op := findOperation(t, doc, path, method)
	rb, ok := op["requestBody"].(map[string]any)
	if !ok {
		t.Fatalf("%s %s requestBody missing", method, path)
	}
	content, ok := rb["content"].(map[string]any)
	if !ok {
		t.Fatalf("%s %s requestBody content missing", method, path)
	}
	media, ok := content["application/json"].(map[string]any)
	if !ok {
		t.Fatalf("%s %s requestBody application/json missing", method, path)
	}
	examples, ok := media["examples"].(map[string]any)
	if !ok {
		t.Fatalf("%s %s requestBody examples missing", method, path)
	}
	return examples
}

func findOperation(t *testing.T, doc map[string]any, path, method string) map[string]any {
	t.Helper()
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("paths missing")
	}
	item, ok := paths[path].(map[string]any)
	if !ok {
		t.Fatalf("path %s missing", path)
	}
	op, ok := item[method].(map[string]any)
	if !ok {
		t.Fatalf("operation %s %s missing", method, path)
	}
	return op
}
