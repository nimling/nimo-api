package converter

import (
	"fmt"
	"path"
	"strings"
)

const (
	examplesRefPrefix  = "#/components/examples/"
	responsesRefPrefix = "#/components/responses/"
	schemasRefPrefix   = "#/components/schemas/"
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
	}

	for _, schema := range n.doc.Components.Schemas {
		walk(schema)
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

const inlineMaxDepth = 256

type inlineCtx struct {
	visiting      map[string]bool
	onPath        map[*Schema]string
	pathPointers  map[*Schema]bool
	nameStack     []string
	nameByPointer map[*Schema]string
	cycleSeen     map[string]bool
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
		visiting:      map[string]bool{},
		onPath:        map[*Schema]string{},
		pathPointers:  map[*Schema]bool{},
		nameByPointer: map[*Schema]string{},
		cycleSeen:     map[string]bool{},
	}
	for name, sch := range n.doc.Components.Schemas {
		if sch != nil {
			ctx.nameByPointer[sch] = name
		}
	}

	walkErr := n.walkOperationSchemas(func(schema **Schema) error {
		inlined, err := n.inlineSchema(*schema, ctx)
		if err != nil {
			return err
		}
		*schema = inlined
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	if len(ctx.cycleSeen) == 0 {
		n.doc.Components.Schemas = nil
		return nil
	}
	kept := make(map[string]*Schema, len(ctx.cycleSeen))
	for name := range ctx.cycleSeen {
		if sch, ok := n.doc.Components.Schemas[name]; ok {
			kept[name] = sch
		}
	}
	n.doc.Components.Schemas = kept
	return nil
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

func (n *OpenAPIConverter) inlineSchema(schema *Schema, ctx *inlineCtx) (*Schema, error) {
	if schema == nil {
		return nil, nil
	}

	if ctx.depth >= inlineMaxDepth {
		return nil, fmt.Errorf("schema inlining exceeded max depth %d at %s", inlineMaxDepth, describeSchemaPath(ctx))
	}
	ctx.depth++
	defer func() { ctx.depth-- }()

	if name, ok := ctx.onPath[schema]; ok {
		ctx.cycleSeen[name] = true
		ref := schemasRefPrefix + name
		return &Schema{Ref: &ref}, nil
	}

	if ctx.pathPointers[schema] {
		if name := outermostName(ctx); name != "" {
			ctx.cycleSeen[name] = true
			ref := schemasRefPrefix + name
			return &Schema{Ref: &ref}, nil
		}
		return &Schema{}, nil
	}

	if schema.Ref != nil && strings.HasPrefix(*schema.Ref, schemasRefPrefix) {
		name := strings.TrimPrefix(*schema.Ref, schemasRefPrefix)
		if ctx.visiting[name] {
			ctx.cycleSeen[name] = true
			return schema, nil
		}
		target := n.doc.Components.Schemas[name]
		if target == nil {
			return nil, fmt.Errorf("unresolved internal schema ref %q", *schema.Ref)
		}
		ctx.visiting[name] = true
		ctx.onPath[target] = name
		ctx.nameStack = append(ctx.nameStack, name)
		expanded, err := n.inlineSchema(cloneSchema(target), ctx)
		ctx.nameStack = ctx.nameStack[:len(ctx.nameStack)-1]
		delete(ctx.visiting, name)
		delete(ctx.onPath, target)
		if err != nil {
			return nil, err
		}
		return expanded, nil
	}

	ctx.pathPointers[schema] = true
	defer delete(ctx.pathPointers, schema)

	if name, ok := ctx.nameByPointer[schema]; ok && !ctx.visiting[name] {
		ctx.visiting[name] = true
		ctx.onPath[schema] = name
		ctx.nameStack = append(ctx.nameStack, name)
		defer func() {
			ctx.nameStack = ctx.nameStack[:len(ctx.nameStack)-1]
			delete(ctx.visiting, name)
			delete(ctx.onPath, schema)
		}()
	}

	if schema.Properties != nil {
		for name, prop := range schema.Properties {
			inlined, err := n.inlineSchema(prop, ctx)
			if err != nil {
				return nil, err
			}
			schema.Properties[name] = inlined
		}
	}
	if schema.Items != nil {
		inlined, err := n.inlineSchema(schema.Items, ctx)
		if err != nil {
			return nil, err
		}
		schema.Items = inlined
	}
	if err := n.inlineSchemaList(schema.AllOf, ctx); err != nil {
		return nil, err
	}
	if err := n.inlineSchemaList(schema.OneOf, ctx); err != nil {
		return nil, err
	}
	if err := n.inlineSchemaList(schema.AnyOf, ctx); err != nil {
		return nil, err
	}
	return schema, nil
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

func (n *OpenAPIConverter) inlineSchemaList(list []*Schema, ctx *inlineCtx) error {
	for i, item := range list {
		inlined, err := n.inlineSchema(item, ctx)
		if err != nil {
			return err
		}
		list[i] = inlined
	}
	return nil
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

func cloneSchema(src *Schema) *Schema {
	if src == nil {
		return nil
	}
	cp := *src
	if src.Properties != nil {
		cp.Properties = make(map[string]*Schema, len(src.Properties))
		for k, v := range src.Properties {
			cp.Properties[k] = v
		}
	}
	if src.Required != nil {
		cp.Required = append([]*string(nil), src.Required...)
	}
	if src.AllOf != nil {
		cp.AllOf = append([]*Schema(nil), src.AllOf...)
	}
	if src.OneOf != nil {
		cp.OneOf = append([]*Schema(nil), src.OneOf...)
	}
	if src.AnyOf != nil {
		cp.AnyOf = append([]*Schema(nil), src.AnyOf...)
	}
	return &cp
}
