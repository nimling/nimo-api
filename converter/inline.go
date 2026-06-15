package converter

import (
	"fmt"
	"os"
	"path"
	"strings"
)

const (
	examplesRefPrefix   = "#/components/examples/"
	responsesRefPrefix  = "#/components/responses/"
	schemasRefPrefix    = "#/components/schemas/"
	parametersRefPrefix = "#/components/parameters/"
)

// NormalizeRefs walks every schema ref in the spec and rewrites file-style
// references (e.g. "./components/ReturnError.yml", "ReturnError.yaml",
// "shared/ReturnError.json") to the standard internal pointer form
// "#/components/schemas/<Name>" when the basename (extension stripped) matches
// a schema in components.schemas. This keeps shared shapes deduplicated in
// components.schemas and lets every consumer follow the standard internal ref.
func (n *OpenAPIConverter) NormalizeRefs() error {
	if n.doc == nil {
		return nil
	}
	if n.doc.Components == nil || n.doc.Components.Schemas == nil {
		return nil
	}

	names := make(map[string]bool, len(n.doc.Components.Schemas))
	for name := range n.doc.Components.Schemas {
		names[name] = true
	}

	rewrite := func(schema *Schema) bool {
		if schema == nil || schema.Ref == nil {
			return false
		}
		ref := *schema.Ref
		if strings.HasPrefix(ref, "#/") {
			return false
		}
		base := path.Base(ref)
		ext := path.Ext(base)
		lower := strings.ToLower(ext)
		if lower != ".yml" && lower != ".yaml" && lower != ".json" {
			return false
		}
		name := strings.TrimSuffix(base, ext)
		if !names[name] {
			return false
		}
		next := schemasRefPrefix + name
		schema.Ref = &next
		return true
	}

	var rewriteRaw func(node interface{})
	rewriteRaw = func(node interface{}) {
		switch v := node.(type) {
		case map[string]interface{}:
			if ref, ok := v["$ref"].(string); ok && !strings.HasPrefix(ref, "#/") {
				base := path.Base(ref)
				ext := path.Ext(base)
				lower := strings.ToLower(ext)
				if lower == ".yml" || lower == ".yaml" || lower == ".json" {
					name := strings.TrimSuffix(base, ext)
					if names[name] {
						v["$ref"] = schemasRefPrefix + name
					}
				}
			}
			for _, child := range v {
				rewriteRaw(child)
			}
		case []interface{}:
			for _, child := range v {
				rewriteRaw(child)
			}
		}
	}

	var walk func(*Schema)
	walk = func(s *Schema) {
		if s == nil {
			return
		}
		rewrite(s)
		for _, prop := range s.Properties {
			walk(prop)
		}
		if s.Items != nil {
			walk(s.Items)
		}
		for _, sub := range s.AllOf {
			walk(sub)
		}
		for _, sub := range s.OneOf {
			walk(sub)
		}
		for _, sub := range s.AnyOf {
			walk(sub)
		}
		rewriteRaw(s.AdditionalProperties)
	}

	for _, schema := range n.doc.Components.Schemas {
		walk(schema)
	}

	for _, response := range n.doc.Components.Responses {
		if response == nil {
			continue
		}
		for _, content := range response.Content {
			if content != nil {
				walk(content.Schema)
			}
		}
	}

	for _, parameter := range n.doc.Components.Parameters {
		if parameter != nil {
			walk(parameter.Schema)
		}
	}

	return n.walkOperationSchemas(func(schema **Schema) error {
		walk(*schema)
		return nil
	})
}

func (n *OpenAPIConverter) InlineExamples() error {
	if n.doc == nil || n.doc.Paths == nil {
		return nil
	}
	if n.doc.Components == nil || n.doc.Components.Examples == nil {
		return nil
	}

	walkErr := n.walkOperationContents(func(content *ResponseContent) error {
		return n.inlineExamplesMap(content.Examples)
	})
	if walkErr != nil {
		return walkErr
	}

	if err := n.inlineParameterExamples(); err != nil {
		return err
	}

	n.doc.Components.Examples = nil
	return nil
}

func (n *OpenAPIConverter) InlineResponses() error {
	if n.doc == nil || n.doc.Paths == nil {
		return nil
	}
	if n.doc.Components == nil || n.doc.Components.Responses == nil {
		return nil
	}

	for _, pathItem := range n.doc.Paths {
		if pathItem == nil {
			continue
		}
		for _, op := range pathItem.Operations() {
			if op == nil || op.Responses == nil {
				continue
			}
			for code, response := range op.Responses {
				if response == nil || response.Ref == nil {
					continue
				}
				resolved := n.resolveInternalResponseRef(*response.Ref)
				if resolved == nil {
					return fmt.Errorf("unresolved internal response ref %q", *response.Ref)
				}
				op.Responses[code] = cloneResponse(resolved)
			}
		}
	}

	n.doc.Components.Responses = nil
	return nil
}

func (n *OpenAPIConverter) InlineParameters() error {
	if n.doc == nil || n.doc.Paths == nil {
		return nil
	}
	if n.doc.Components == nil || n.doc.Components.Parameters == nil {
		return nil
	}

	for _, pathItem := range n.doc.Paths {
		if pathItem == nil {
			continue
		}
		for i, param := range pathItem.Parameters {
			if param == nil || param.Ref == nil {
				continue
			}
			resolved := n.resolveInternalParameterRef(*param.Ref)
			if resolved == nil {
				return fmt.Errorf("unresolved internal parameter ref %q", *param.Ref)
			}
			pathItem.Parameters[i] = cloneParameter(resolved)
		}
		for _, op := range pathItem.Operations() {
			if op == nil {
				continue
			}
			for i, param := range op.Parameters {
				if param == nil || param.Ref == nil {
					continue
				}
				resolved := n.resolveInternalParameterRef(*param.Ref)
				if resolved == nil {
					return fmt.Errorf("unresolved internal parameter ref %q", *param.Ref)
				}
				op.Parameters[i] = cloneParameter(resolved)
			}
		}
	}

	n.doc.Components.Parameters = nil
	return nil
}

const inlineMaxDepth = 256

type inlineCtx struct {
	nameByPointer map[*Schema]string
	ancestors     []*Schema
	nameStack     []string
	depth         int
}

func (n *OpenAPIConverter) InlineSchemas() error {
	if n.doc == nil || n.doc.Paths == nil {
		return nil
	}
	if n.doc.Components == nil || n.doc.Components.Schemas == nil {
		return nil
	}

	ctx := &inlineCtx{
		nameByPointer: map[*Schema]string{},
	}
	for name, sch := range n.doc.Components.Schemas {
		if sch != nil {
			ctx.nameByPointer[sch] = name
		}
	}

	walkErr := n.walkOperationSchemas(func(schemaPtr **Schema) error {
		ctx.ancestors = ctx.ancestors[:0]
		ctx.nameStack = ctx.nameStack[:0]
		ctx.depth = 0
		rewritten, err := n.dedupeEntry(*schemaPtr, ctx)
		if err != nil {
			return err
		}
		*schemaPtr = rewritten
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	for name, sch := range n.doc.Components.Schemas {
		if sch == nil {
			continue
		}
		ctx.ancestors = append(ctx.ancestors[:0], sch)
		ctx.nameStack = append(ctx.nameStack[:0], name)
		ctx.depth = 0
		if err := n.dedupeSchema(sch, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (n *OpenAPIConverter) dedupeEntry(schema *Schema, ctx *inlineCtx) (*Schema, error) {
	if schema == nil {
		return nil, nil
	}
	if name, ok := ctx.nameByPointer[schema]; ok {
		r := schemasRefPrefix + name
		return &Schema{Ref: &r}, nil
	}
	if err := n.dedupeSchema(schema, ctx); err != nil {
		return nil, err
	}
	return schema, nil
}

func (n *OpenAPIConverter) dedupeSchema(schema *Schema, ctx *inlineCtx) error {
	if schema == nil {
		return nil
	}
	if ctx.depth >= inlineMaxDepth {
		return fmt.Errorf("schema dedupe exceeded max depth %d at %s", inlineMaxDepth, describeSchemaPath(ctx))
	}
	ctx.depth++
	defer func() { ctx.depth-- }()

	ctx.ancestors = append(ctx.ancestors, schema)
	name, isComponent := ctx.nameByPointer[schema]
	if isComponent {
		ctx.nameStack = append(ctx.nameStack, name)
	}
	defer func() {
		ctx.ancestors = ctx.ancestors[:len(ctx.ancestors)-1]
		if isComponent {
			ctx.nameStack = ctx.nameStack[:len(ctx.nameStack)-1]
		}
	}()

	if schema.Properties != nil {
		for k, prop := range schema.Properties {
			replaced, err := n.dedupeChild(prop, ctx)
			if err != nil {
				return err
			}
			schema.Properties[k] = replaced
		}
	}
	if schema.Items != nil {
		replaced, err := n.dedupeChild(schema.Items, ctx)
		if err != nil {
			return err
		}
		schema.Items = replaced
	}
	for i, sub := range schema.AllOf {
		replaced, err := n.dedupeChild(sub, ctx)
		if err != nil {
			return err
		}
		schema.AllOf[i] = replaced
	}
	for i, sub := range schema.OneOf {
		replaced, err := n.dedupeChild(sub, ctx)
		if err != nil {
			return err
		}
		schema.OneOf[i] = replaced
	}
	for i, sub := range schema.AnyOf {
		replaced, err := n.dedupeChild(sub, ctx)
		if err != nil {
			return err
		}
		schema.AnyOf[i] = replaced
	}
	return nil
}

func (n *OpenAPIConverter) dedupeChild(child *Schema, ctx *inlineCtx) (*Schema, error) {
	if child == nil {
		return nil, nil
	}
	if child.Ref != nil && strings.HasPrefix(*child.Ref, schemasRefPrefix) {
		return child, nil
	}
	if name, ok := ctx.nameByPointer[child]; ok {
		r := schemasRefPrefix + name
		return &Schema{Ref: &r}, nil
	}
	for _, anc := range ctx.ancestors {
		if anc == child {
			if name := outermostName(ctx); name != "" {
				fmt.Fprintf(os.Stderr, "[nimo] cycle: anonymous nesting at %s, replacing with $ref %s\n", describeSchemaPath(ctx), schemasRefPrefix+name)
				r := schemasRefPrefix + name
				return &Schema{Ref: &r}, nil
			}
			fmt.Fprintf(os.Stderr, "[nimo] cycle: anonymous nesting at %s with no named ancestor, replacing with empty schema\n", describeSchemaPath(ctx))
			return &Schema{}, nil
		}
	}
	if err := n.dedupeSchema(child, ctx); err != nil {
		return nil, err
	}
	return child, nil
}

func (n *OpenAPIConverter) walkOperationContents(fn func(content *ResponseContent) error) error {
	for _, pathItem := range n.doc.Paths {
		if pathItem == nil {
			continue
		}
		for _, op := range pathItem.Operations() {
			if op == nil {
				continue
			}
			if op.RequestBody != nil {
				for _, content := range op.RequestBody.Content {
					if content == nil {
						continue
					}
					if err := fn(content); err != nil {
						return err
					}
				}
			}
			for _, response := range op.Responses {
				if response == nil {
					continue
				}
				for _, content := range response.Content {
					if content == nil {
						continue
					}
					if err := fn(content); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (n *OpenAPIConverter) inlineParameterExamples() error {
	for _, pathItem := range n.doc.Paths {
		if pathItem == nil {
			continue
		}
		for _, param := range pathItem.Parameters {
			if err := n.inlineExamplesMap(param.Examples); err != nil {
				return err
			}
		}
		for _, op := range pathItem.Operations() {
			if op == nil {
				continue
			}
			for _, param := range op.Parameters {
				if err := n.inlineExamplesMap(param.Examples); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (n *OpenAPIConverter) inlineExamplesMap(examples map[string]*Example) error {
	if examples == nil {
		return nil
	}
	for name, ex := range examples {
		if ex == nil || ex.Ref == nil {
			continue
		}
		resolved := n.resolveInternalExampleRef(*ex.Ref)
		if resolved == nil {
			return fmt.Errorf("unresolved internal example ref %q", *ex.Ref)
		}
		examples[name] = cloneExample(resolved)
	}
	return nil
}

func (n *OpenAPIConverter) resolveInternalExampleRef(ref string) *Example {
	if !strings.HasPrefix(ref, examplesRefPrefix) {
		return nil
	}
	name := strings.TrimPrefix(ref, examplesRefPrefix)
	if n.doc.Components == nil || n.doc.Components.Examples == nil {
		return nil
	}
	return n.doc.Components.Examples[name]
}

func (n *OpenAPIConverter) resolveInternalResponseRef(ref string) *Response {
	if !strings.HasPrefix(ref, responsesRefPrefix) {
		return nil
	}
	name := strings.TrimPrefix(ref, responsesRefPrefix)
	if n.doc.Components == nil || n.doc.Components.Responses == nil {
		return nil
	}
	return n.doc.Components.Responses[name]
}

func (n *OpenAPIConverter) resolveInternalParameterRef(ref string) *Parameter {
	if !strings.HasPrefix(ref, parametersRefPrefix) {
		return nil
	}
	name := strings.TrimPrefix(ref, parametersRefPrefix)
	if n.doc.Components == nil || n.doc.Components.Parameters == nil {
		return nil
	}
	return n.doc.Components.Parameters[name]
}

func (n *OpenAPIConverter) walkOperationSchemas(fn func(schema **Schema) error) error {
	for _, pathItem := range n.doc.Paths {
		if pathItem == nil {
			continue
		}
		for _, param := range pathItem.Parameters {
			if param != nil && param.Schema != nil {
				if err := fn(&param.Schema); err != nil {
					return err
				}
			}
		}
		for _, op := range pathItem.Operations() {
			if op == nil {
				continue
			}
			for _, param := range op.Parameters {
				if param != nil && param.Schema != nil {
					if err := fn(&param.Schema); err != nil {
						return err
					}
				}
			}
			if op.RequestBody != nil {
				for _, content := range op.RequestBody.Content {
					if content != nil && content.Schema != nil {
						if err := fn(&content.Schema); err != nil {
							return err
						}
					}
				}
			}
			for _, response := range op.Responses {
				if response == nil {
					continue
				}
				for _, content := range response.Content {
					if content != nil && content.Schema != nil {
						if err := fn(&content.Schema); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

func outermostName(ctx *inlineCtx) string {
	for i := len(ctx.nameStack) - 1; i >= 0; i-- {
		if ctx.nameStack[i] != "" {
			return ctx.nameStack[i]
		}
	}
	return ""
}

func describeSchemaPath(ctx *inlineCtx) string {
	if len(ctx.nameStack) == 0 {
		return "<root>"
	}
	return strings.Join(ctx.nameStack, " -> ")
}

func cloneExample(src *Example) *Example {
	if src == nil {
		return nil
	}
	cp := *src
	return &cp
}

func cloneResponse(src *Response) *Response {
	if src == nil {
		return nil
	}
	cp := *src
	if src.Content != nil {
		cp.Content = make(map[string]*ResponseContent, len(src.Content))
		for k, v := range src.Content {
			if v == nil {
				continue
			}
			content := *v
			cp.Content[k] = &content
		}
	}
	return &cp
}

func cloneParameter(src *Parameter) *Parameter {
	if src == nil {
		return nil
	}
	cp := *src
	cp.Ref = nil
	if src.Examples != nil {
		cp.Examples = make(map[string]*Example, len(src.Examples))
		for k, v := range src.Examples {
			cp.Examples[k] = cloneExample(v)
		}
	}
	return &cp
}
