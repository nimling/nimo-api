package converter

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// DereferenceAll walks the spec and replaces every remaining `$ref` pointing
// at the components map with a deep-copy of the referenced node. Cycles are
// broken by leaving the inner-most `$ref` intact when the same pointer is
// already on the resolution path. After this method runs the now-empty
// component maps (`schemas`, `responses`, `examples`, `parameters`) are
// dropped from the document, so the result is a single self-contained spec
// without internal refs.
func (n *OpenAPIConverter) DereferenceAll() error {
	doc, err := n.docAsAnyMap()
	if err != nil {
		return err
	}

	d := &derefRun{root: doc}
	d.expand(doc, map[string]bool{})

	if components, ok := doc["components"].(map[string]interface{}); ok {
		delete(components, "schemas")
		delete(components, "responses")
		delete(components, "examples")
		delete(components, "parameters")
		if len(components) == 0 {
			delete(doc, "components")
		}
	}

	rebuilt, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal deref'd doc: %w", err)
	}
	var fresh OpenAPIDoc
	if err := yaml.Unmarshal(rebuilt, &fresh); err != nil {
		return fmt.Errorf("failed to unmarshal deref'd doc: %w", err)
	}
	n.doc = &fresh
	return nil
}

type derefRun struct {
	root map[string]interface{}
}

func (d *derefRun) resolve(ref string, stack map[string]bool) (interface{}, bool) {
	if !strings.HasPrefix(ref, "#/") {
		return nil, false
	}
	if stack[ref] {
		return nil, false
	}
	stack[ref] = true
	defer delete(stack, ref)

	segments := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
	var cursor interface{} = d.root
	for _, seg := range segments {
		seg = strings.ReplaceAll(seg, "~1", "/")
		seg = strings.ReplaceAll(seg, "~0", "~")
		m, ok := cursor.(map[string]interface{})
		if !ok {
			return nil, false
		}
		cursor, ok = m[seg]
		if !ok {
			return nil, false
		}
	}
	walked := deepCopyJSON(cursor)
	d.expand(walked, stack)
	return walked, true
}

func (d *derefRun) expand(node interface{}, stack map[string]bool) {
	switch v := node.(type) {
	case map[string]interface{}:
		if ref, ok := v["$ref"].(string); ok {
			resolved, replaced := d.resolve(ref, stack)
			if replaced {
				delete(v, "$ref")
				if m, ok := resolved.(map[string]interface{}); ok {
					for k, val := range m {
						if _, present := v[k]; !present {
							v[k] = val
						}
					}
				}
			}
		}
		for _, child := range v {
			d.expand(child, stack)
		}
	case []interface{}:
		for _, child := range v {
			d.expand(child, stack)
		}
	}
}

func deepCopyJSON(node interface{}) interface{} {
	switch v := node.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for k, val := range v {
			out[k] = deepCopyJSON(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, val := range v {
			out[i] = deepCopyJSON(val)
		}
		return out
	}
	return node
}
