package converter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var splitSlugReplacer = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func (n *OpenAPIConverter) WriteSplitOpenAPISpec(outputDir, dirName, fileName string) error {
	if dirName == "" {
		dirName = fmt.Sprintf("%sspec", strings.TrimSuffix(n.FilePrefix, "."))
	}
	if fileName == "" {
		fileName = "spec.json"
	}
	if !strings.HasSuffix(fileName, ".json") {
		fileName += ".json"
	}
	rootDir := filepath.Join(outputDir, n.CommonPrefix, dirName)
	specPath := filepath.Join(rootDir, fileName)
	operationsDir := filepath.Join(rootDir, "operations")
	schemasDir := filepath.Join(rootDir, "schemas")

	if err := os.MkdirAll(operationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create operations dir: %w", err)
	}
	if err := os.MkdirAll(schemasDir, 0755); err != nil {
		return fmt.Errorf("failed to create schemas dir: %w", err)
	}

	doc, err := n.docAsAnyMap()
	if err != nil {
		return err
	}

	if err := splitOutSchemas(doc, schemasDir); err != nil {
		return err
	}

	if err := splitOutOperations(doc, operationsDir); err != nil {
		return err
	}

	rewriteSchemaRefs(doc, "./schemas/")

	out, err := json.MarshalIndent(doc, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal spec.json: %w", err)
	}
	if err := os.WriteFile(specPath, out, 0644); err != nil {
		return fmt.Errorf("failed to write spec.json: %w", err)
	}
	LogWrite(specPath, len(out))

	fmt.Printf("Successfully wrote split spec to %s\n", rootDir)
	return nil
}

func (n *OpenAPIConverter) docAsAnyMap() (map[string]interface{}, error) {
	yamlData, err := yaml.Marshal(n.doc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal doc to yaml: %w", err)
	}
	var raw interface{}
	if err := yaml.Unmarshal(yamlData, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml to map: %w", err)
	}
	root, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("root yaml is not a map")
	}
	return root, nil
}

func splitOutSchemas(doc map[string]interface{}, schemasDir string) error {
	components, ok := doc["components"].(map[string]interface{})
	if !ok {
		return nil
	}
	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		return nil
	}
	for name, schema := range schemas {
		body, ok := schema.(map[string]interface{})
		if !ok {
			continue
		}
		rewriteSchemaRefs(body, "./")
		out, err := json.MarshalIndent(body, "", "    ")
		if err != nil {
			return fmt.Errorf("failed to marshal schema %s: %w", name, err)
		}
		dest := filepath.Join(schemasDir, splitSafeName(name)+".json")
		if err := os.WriteFile(dest, out, 0644); err != nil {
			return fmt.Errorf("failed to write schema %s: %w", name, err)
		}
		LogWrite(dest, len(out))
	}
	stubs := map[string]interface{}{}
	for name := range schemas {
		stubs[name] = map[string]string{"$ref": "./schemas/" + splitSafeName(name) + ".json"}
	}
	components["schemas"] = stubs
	return nil
}

func splitOutOperations(doc map[string]interface{}, operationsDir string) error {
	paths, ok := doc["paths"].(map[string]interface{})
	if !ok {
		return nil
	}
	methods := []string{"get", "post", "put", "patch", "delete", "head", "options"}
	for path, pathItemRaw := range paths {
		pathItem, ok := pathItemRaw.(map[string]interface{})
		if !ok {
			continue
		}
		for _, method := range methods {
			opRaw, ok := pathItem[method]
			if !ok {
				continue
			}
			op, ok := opRaw.(map[string]interface{})
			if !ok {
				continue
			}
			opId, _ := op["operationId"].(string)
			if opId == "" {
				opId = method + "-" + splitSafeName(path)
			}
			rewriteSchemaRefs(op, "../schemas/")
			out, err := json.MarshalIndent(op, "", "    ")
			if err != nil {
				return fmt.Errorf("failed to marshal operation %s: %w", opId, err)
			}
			dest := filepath.Join(operationsDir, splitSafeName(opId)+".json")
			if err := os.WriteFile(dest, out, 0644); err != nil {
				return fmt.Errorf("failed to write operation %s: %w", opId, err)
			}
			LogWrite(dest, len(out))
			pathItem[method] = map[string]string{"$ref": "./operations/" + splitSafeName(opId) + ".json"}
		}
	}
	return nil
}

func rewriteSchemaRefs(node interface{}, prefix string) {
	switch v := node.(type) {
	case map[string]interface{}:
		if ref, ok := v["$ref"].(string); ok {
			if strings.HasPrefix(ref, "#/components/schemas/") {
				name := strings.TrimPrefix(ref, "#/components/schemas/")
				v["$ref"] = prefix + splitSafeName(name) + ".json"
			}
		}
		for _, child := range v {
			rewriteSchemaRefs(child, prefix)
		}
	case []interface{}:
		for _, child := range v {
			rewriteSchemaRefs(child, prefix)
		}
	}
}

func splitSafeName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "_"
	}
	return splitSlugReplacer.ReplaceAllString(trimmed, "-")
}
