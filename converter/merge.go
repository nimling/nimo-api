package converter

import "strings"

func (n *OpenAPIConverter) MergeResponsesInline() error {
	if n.doc.Paths == nil {
		return nil
	}

	for _, pathItem := range n.doc.Paths {
		for _, op := range pathItem.Operations() {
			if op.Responses == nil {
				continue
			}

			for _, response := range op.Responses {
				if response.Content == nil {
					continue
				}

				for _, content := range response.Content {
					if content.Schema == nil || content.Schema.AllOf == nil {
						continue
					}

					mergedSchema, err := mergeAllOf(content.Schema.AllOf, n.doc.Components)
					if err != nil {
						return err
					}
					content.Schema = mergedSchema
				}
			}
		}
	}
	return nil
}

func mergeAllOf(schemas []*Schema, components *Components) (*Schema, error) {
	if len(schemas) == 0 {
		return nil, nil
	}

	// Resolve and use first schema as base
	baseSchema := schemas[0]
	if baseSchema.Ref != nil {
		baseSchema = components.GetSchema(*baseSchema.Ref)
	}

	// Create a new schema copying the base
	result := &Schema{
		Type:        baseSchema.Type,
		Properties:  make(map[string]*Schema),
		Description: baseSchema.Description,
		Required:    baseSchema.Required,
	}

	// Copy properties
	for k, v := range baseSchema.Properties {
		result.Properties[k] = v
	}

	// Process remaining schemas
	for _, schema := range schemas[1:] {
		if schema.Ref != nil {
			schema = components.GetSchema(*schema.Ref)
		}

		// Merge properties
		if schema.Properties != nil {
			for propName, propSchema := range schema.Properties {
				result.Properties[propName] = propSchema
			}
		}

		// Merge required fields
		if schema.Required != nil {
			result.Required = append(result.Required, schema.Required...)
		}

		// Update type if specified
		if len(schema.Type) > 0 {
			result.Type = schema.Type
		}
	}

	return result, nil
}

func (c *Components) GetSchema(name string) *Schema {
	if strings.HasPrefix(name, "#/components/schemas/") {
		name = strings.TrimPrefix(name, "#/components/schemas/")
	}

	return c.Schemas[name]
}
