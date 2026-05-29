package converter

import (
	"fmt"
	"strings"
)

const (
	examplesRefPrefix  = "#/components/examples/"
	responsesRefPrefix = "#/components/responses/"
	schemasRefPrefix   = "#/components/schemas/"
)

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

func (n *OpenAPIConverter) InlineSchemas() error {
	if n.doc == nil || n.doc.Paths == nil {
		return nil
	}
	if n.doc.Components == nil || n.doc.Components.Schemas == nil {
		return nil
	}

	visiting := map[string]bool{}

	walkErr := n.walkOperationSchemas(func(schema **Schema) error {
		inlined, err := n.inlineSchema(*schema, visiting)
		if err != nil {
			return err
		}
		*schema = inlined
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	n.doc.Components.Schemas = nil
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

func (n *OpenAPIConverter) inlineSchema(schema *Schema, visiting map[string]bool) (*Schema, error) {
	if schema == nil {
		return nil, nil
	}

	if schema.Ref != nil && strings.HasPrefix(*schema.Ref, schemasRefPrefix) {
		name := strings.TrimPrefix(*schema.Ref, schemasRefPrefix)
		if visiting[name] {
			return schema, nil
		}
		target := n.doc.Components.Schemas[name]
		if target == nil {
			return nil, fmt.Errorf("unresolved internal schema ref %q", *schema.Ref)
		}
		visiting[name] = true
		expanded, err := n.inlineSchema(cloneSchema(target), visiting)
		delete(visiting, name)
		if err != nil {
			return nil, err
		}
		return expanded, nil
	}

	if schema.Properties != nil {
		for name, prop := range schema.Properties {
			inlined, err := n.inlineSchema(prop, visiting)
			if err != nil {
				return nil, err
			}
			schema.Properties[name] = inlined
		}
	}
	if schema.Items != nil {
		inlined, err := n.inlineSchema(schema.Items, visiting)
		if err != nil {
			return nil, err
		}
		schema.Items = inlined
	}
	if err := n.inlineSchemaList(schema.AllOf, visiting); err != nil {
		return nil, err
	}
	if err := n.inlineSchemaList(schema.OneOf, visiting); err != nil {
		return nil, err
	}
	if err := n.inlineSchemaList(schema.AnyOf, visiting); err != nil {
		return nil, err
	}
	return schema, nil
}

func (n *OpenAPIConverter) inlineSchemaList(list []*Schema, visiting map[string]bool) error {
	for i, item := range list {
		inlined, err := n.inlineSchema(item, visiting)
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
