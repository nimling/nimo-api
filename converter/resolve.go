package converter

import (
	"fmt"
	"github.com/nimling/nimo-api/utils"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

func (n *OpenAPIConverter) ResolveExternalRefs() error {
	// Initialize component register if not already done
	if n.doc.Components == nil {
		n.doc.Components = &Components{
			SecuritySchemes: map[string]*SecurityScheme{},
			Parameters:      map[string]*Parameter{},
			Schemas:         map[string]*Schema{},
			Register:        ReferenceRegister{},
		}
	}

	// Iteratively resolve references until no external refs remain
	maxIterations := 10 // Prevent infinite loops
	for i := 0; i < maxIterations; i++ {
		externalRefsRemain := false

		// First resolve components
		if err := n.resolveComponentRefs(n.doc.Components); err != nil {
			return fmt.Errorf("failed to resolve component references: %w", err)
		}

		// Then resolve path items
		if n.doc.Paths != nil {
			for key, pathItem := range n.doc.Paths {
				if err := n.resolvePathItemRefs(pathItem); err != nil {
					return fmt.Errorf("failed to resolve refs in path %s: %w", key, err)
				}
			}
		}

		// Check if any external references remain
		externalRefsRemain = n.hasExternalRefs()

		if !externalRefsRemain {
			// No more external refs, we're done
			break
		}

		if i == maxIterations-1 {
			return fmt.Errorf("failed to resolve all external references after %d iterations", maxIterations)
		}
	}

	return nil
}

func (n *OpenAPIConverter) resolvePathItemRefs(pathItem *PathItem) error {
	if pathItem == nil {
		return nil
	}

	// Process path parameters if any
	if pathItem.Parameters != nil {
		var err error
		pathItem.Parameters, err = n.resolveParameterRefs(pathItem.Parameters, n.filePath)
		if err != nil {
			return fmt.Errorf("failed to resolve path parameters: %w", err)
		}
	}

	// Process operations
	for method, op := range pathItem.Operations() {
		if op == nil {
			continue
		}

		// Handle operation reference
		relPath := n.filePath
		if op.Ref != nil && isExternalRef(*op.Ref) {
			filePath := resolveRefPath(n.filePath, *op.Ref)
			resolved, err := loadExternalRef[Operation](filePath)
			if err != nil {
				return fmt.Errorf("failed to load external operation ref: %w", err)
			}
			op = resolved
			relPath = filePath
		}

		// Process operation parameters
		if op.Parameters != nil {
			var err error
			op.Parameters, err = n.resolveParameterRefs(op.Parameters, relPath)
			if err != nil {
				return fmt.Errorf("failed to resolve operation parameters: %w", err)
			}
		}

		// Process request body
		if op.RequestBody != nil {
			if op.RequestBody.Ref != nil && isExternalRef(*op.RequestBody.Ref) {
				requestRelPath := resolveRefPath(relPath, *op.RequestBody.Ref)
				resolved, err := loadExternalRef[RequestBody](requestRelPath)
				if err != nil {
					return fmt.Errorf("failed to load external request body ref: %w", err)
				}
				op.RequestBody = resolved
			}

			// Process request body content schemas
			if op.RequestBody.Content != nil {
				for mediaType, content := range op.RequestBody.Content {
					if content == nil || content.Schema == nil {
						continue
					}

					if err := content.Schema.resolveExternalRefs(n.doc.Components, relPath); err != nil {
						return fmt.Errorf("failed to resolve request body schema for %s: %w", mediaType, err)
					}
				}
			}
		}

		// Process responses
		if op.Responses != nil {
			for code, response := range op.Responses {
				if response == nil {
					continue
				}

				if response.Ref != nil && isExternalRef(*response.Ref) {
					responseRelPath := resolveRefPath(relPath, *response.Ref)
					resolved, err := loadExternalRef[Response](responseRelPath)
					if err != nil {
						return fmt.Errorf("failed to load external response ref: %w", err)
					}
					op.Responses[code] = resolved
				}

				// Process response content schemas
				if response.Content != nil {
					for mediaType, content := range response.Content {
						if content == nil || content.Schema == nil {
							continue
						}

						if err := content.Schema.resolveExternalRefs(n.doc.Components, relPath); err != nil {
							return fmt.Errorf("failed to resolve response schema for %s: %w", mediaType, err)
						}
					}
				}
			}
		}

		// Update the operation in path item
		pathItem.SetMethodOperation(method, op)
	}

	return nil
}

func (r *Schema) resolveExternalRefs(components *Components, relPath string) error {
	// Handle direct reference
	if r.Ref != nil && isExternalRef(*r.Ref) {
		refFilePath := resolveRefPath(relPath, *r.Ref)

		// Check if we've already processed this reference
		if existingRef, ok := components.Register[refFilePath]; ok {
			// Just update to internal reference
			r.Ref = &existingRef
			return nil
		}

		resolved, err := loadExternalRef[Schema](refFilePath)
		if err != nil {
			return fmt.Errorf("failed to load external ref: %w", err)
		}

		comp := components.PutRegister("schemas", refFilePath)
		r.Ref = &comp.Identifier

		if err := resolved.resolveExternalRefs(components, refFilePath); err != nil {
			return err
		}

		// Define the component if it does not exist
		if components.Schemas[comp.Name] == nil {
			components.Schemas[comp.Name] = resolved
		}

		return nil
	}

	// Handle allOf array - keeping the original context for each item
	if r.AllOf != nil {
		for i, schema := range r.AllOf {
			if schema == nil {
				continue
			}

			// Process each allOf schema with the current context path
			if err := schema.resolveExternalRefs(components, relPath); err != nil {
				return fmt.Errorf("failed to resolve allOf[%d]: %w", i, err)
			}
		}
	}

	// Process properties - use the same context path
	if r.Properties != nil {
		for propName, prop := range r.Properties {
			if prop == nil {
				continue
			}

			if err := prop.resolveExternalRefs(components, relPath); err != nil {
				return fmt.Errorf("failed to resolve property '%s': %w", propName, err)
			}
		}
	}

	// Process items - use the same context path
	if r.Items != nil {
		if err := r.Items.resolveExternalRefs(components, relPath); err != nil {
			return fmt.Errorf("failed to resolve array items: %w", err)
		}
	}

	return nil
}

func (n *OpenAPIConverter) resolveParameterRefs(params []*Parameter, relPath string) ([]*Parameter, error) {
	if params == nil {
		return nil, nil
	}

	var result []*Parameter

	for _, param := range params {
		if param == nil {
			continue
		}

		// Handle external reference
		if param.Ref != nil && isExternalRef(*param.Ref) {
			refFilePath := resolveRefPath(relPath, *param.Ref)

			// Check if this reference is already registered
			if n.doc.Components != nil && n.doc.Components.Register != nil {
				if existingRef, exists := n.doc.Components.Register[refFilePath]; exists {
					// Add reference to the existing component
					result = append(result, &Parameter{
						Ref: utils.StringPtr(existingRef),
					})
					continue
				}
			}

			// Load the referenced parameter file
			resolved, err := loadExternalRef[Parameter](refFilePath)
			if err != nil {
				return nil, fmt.Errorf("failed to load external parameter ref %s: %w", *param.Ref, err)
			}

			// If the loaded parameter has a schema with explode and has properties
			if resolved.Schema != nil && resolved.Schema.Properties != nil {
				// Check for explode possibility
				exploded := false
				if resolved.Schema.Type != nil && *resolved.Schema.Type == "object" {
					exploded = true
				}

				if exploded {
					// Determine base name for component (use filename without extension)
					baseName := filepath.Base(strings.TrimSuffix(refFilePath, filepath.Ext(refFilePath)))

					// Create individual parameters for each property
					for propName, propSchema := range resolved.Schema.Properties {
						explodedParam := &Parameter{
							Name:     propName,
							In:       resolved.In,
							Required: false,
							Schema:   propSchema,
							Example:  propSchema.Example,
						}

						if propSchema.Description != nil {
							explodedParam.Description = *propSchema.Description
						} else {
							explodedParam.Description = "Resolved from " + propName
						}

						// Check if this property is in the original schema's required list
						if resolved.Schema.Required != nil {
							for _, requiredProp := range resolved.Schema.Required {
								if *requiredProp == propName {
									explodedParam.Required = true
									break
								}
							}
						}

						// Generate a unique component name using camel case
						componentName := generateComponentName(baseName, propName)

						// Ensure components is initialized
						if n.doc.Components == nil {
							n.doc.Components = &Components{
								Parameters: make(map[string]*Parameter),
								Register:   ReferenceRegister{},
							}
						}

						// Register the exploded parameter
						n.doc.Components.Parameters[componentName] = explodedParam
						n.doc.Components.PutRegister("parameters", refFilePath+"#"+componentName)

						// Add a reference to the new component parameter
						result = append(result, &Parameter{
							Ref: utils.StringPtr(fmt.Sprintf("#/components/parameters/%s", componentName)),
						})
					}
				} else {
					// Not explodable, just register as a normal component
					componentName := filepath.Base(strings.TrimSuffix(refFilePath, filepath.Ext(refFilePath)))

					if n.doc.Components == nil {
						n.doc.Components = &Components{
							Parameters: make(map[string]*Parameter),
							Register:   ReferenceRegister{},
						}
					}

					n.doc.Components.Parameters[componentName] = resolved
					n.doc.Components.PutRegister("parameters", refFilePath)

					// Add reference to the parameter
					result = append(result, &Parameter{
						Ref: utils.StringPtr(fmt.Sprintf("#/components/parameters/%s", componentName)),
					})
				}
			} else {
				// No properties to explode, treat as normal parameter
				componentName := filepath.Base(strings.TrimSuffix(refFilePath, filepath.Ext(refFilePath)))

				if n.doc.Components == nil {
					n.doc.Components = &Components{
						Parameters: make(map[string]*Parameter),
						Register:   ReferenceRegister{},
					}
				}

				n.doc.Components.Parameters[componentName] = resolved
				n.doc.Components.PutRegister("parameters", refFilePath)

				// Add reference to the parameter
				result = append(result, &Parameter{
					Ref: utils.StringPtr(fmt.Sprintf("#/components/parameters/%s", componentName)),
				})
			}
		} else {
			// Not an external reference, keep as is
			result = append(result, param)
		}
	}

	return result, nil
}

func generateComponentName(baseName string, propName string) string {
	// Split the base name by delimiters
	parts := strings.FieldsFunc(baseName, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})

	// Capitalize each part
	for i, part := range parts {
		parts[i] = strings.Title(strings.ToLower(part))
	}

	// Capitalize the first letter of the property name
	propParts := strings.FieldsFunc(propName, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	for i, part := range propParts {
		propParts[i] = strings.Title(strings.ToLower(part))
	}

	// Combine base name parts and property name parts
	return strings.Join(parts, "") + strings.Join(propParts, "")
}

func (n *OpenAPIConverter) resolveComponentRefs(components *Components) error {
	if components == nil {
		return nil
	}

	if components.SecuritySchemes != nil {
		for key, comp := range components.SecuritySchemes {
			if comp.Ref != nil && isExternalRef(*comp.Ref) {
				filePath := resolveRefPath(n.filePath, *comp.Ref)
				res, err := loadExternalRef[SecurityScheme](filePath)
				if err != nil {
					return fmt.Errorf("failed to load external ref: %s, error: %w", err)
				}

				components.PutRegister("securitySchemes", filePath)
				components.SecuritySchemes[key] = res
			}
		}
	}

	if components.Parameters != nil {
		for key, comp := range components.Parameters {
			if comp.Ref != nil && isExternalRef(*comp.Ref) {
				filePath := resolveRefPath(n.filePath, *comp.Ref)
				res, err := loadExternalRef[Parameter](filePath)
				if err != nil {
					return fmt.Errorf("failed to load external ref: %s, error: %w", err)
				}

				components.PutRegister("parameters", filePath)
				components.Parameters[key] = res
			}
		}
	}

	relPath := n.filePath
	if components.Schemas != nil {
		for key, comp := range components.Schemas {
			if comp.Ref != nil && isExternalRef(*comp.Ref) {
				relPath = resolveRefPath(n.filePath, *comp.Ref)
				res, err := loadExternalRef[Schema](relPath)
				if err != nil {
					return fmt.Errorf("failed to load external ref: %s, error: %w", err)
				}

				components.PutRegister("schemas", relPath)
				comp = res
			} else {
				def := components.PutRegister("schemas", key)
				if def != nil {
					relPath = def.FilePath
				}
			}

			if err := comp.resolveExternalRefs(n.doc.Components, relPath); err != nil {
				return fmt.Errorf("failed to resolve external refs: %w", err)
			}

			components.Schemas[key] = comp
		}
	}

	return nil
}

func resolveRefPath(specPath, refPath string) string {
	filePath, _ := splitRefPath(refPath)

	if !strings.HasPrefix(filePath, "./") && !strings.HasPrefix(filePath, "../") {
		return filePath
	}

	baseDir := filepath.Dir(specPath)
	absPath := filepath.Join(baseDir, filePath)

	return filepath.Clean(absPath)
}

func loadExternalRef[T any](filePath string) (*T, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filePath, err)
	}

	var result T
	if err = yaml.Unmarshal(content, &result); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
	}

	return &result, nil
}

func splitRefPath(refPath string) (string, string) {
	parts := strings.SplitN(refPath, "#", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return refPath, ""
}

func isExternalRef(refPath string) bool {
	// Check if the reference points to an external file
	// External refs typically start with ./ or ../ or are absolute paths
	return strings.HasPrefix(refPath, "./") ||
		strings.HasPrefix(refPath, "../") ||
		strings.HasPrefix(refPath, "/") ||
		strings.Contains(refPath, "://")
}

// Add helper method to check for remaining external refs
func (n *OpenAPIConverter) hasExternalRefs() bool {
	// Check components for external refs
	if n.doc.Components != nil {
		if hasExternalRefsInComponents(n.doc.Components) {
			return true
		}
	}

	// Check paths for external refs
	if n.doc.Paths != nil {
		for _, pathItem := range n.doc.Paths {
			if hasExternalRefsInPathItem(pathItem) {
				return true
			}
		}
	}

	return false
}

// Helper functions to check for external refs
func hasExternalRefsInComponents(components *Components) bool {
	if components == nil {
		return false
	}

	// Check schemas
	if components.Schemas != nil {
		for _, schema := range components.Schemas {
			if hasExternalRefsInSchema(schema) {
				return true
			}
		}
	}

	// Check parameters
	if components.Parameters != nil {
		for _, param := range components.Parameters {
			if param.Ref != nil && isExternalRef(*param.Ref) {
				return true
			}
			if param.Schema != nil && hasExternalRefsInSchema(param.Schema) {
				return true
			}
		}
	}

	// Check security schemes
	if components.SecuritySchemes != nil {
		for _, scheme := range components.SecuritySchemes {
			if scheme.Ref != nil && isExternalRef(*scheme.Ref) {
				return true
			}
		}
	}

	return false
}

func hasExternalRefsInSchema(schema *Schema) bool {
	if schema == nil {
		return false
	}

	// Check direct ref
	if schema.Ref != nil && isExternalRef(*schema.Ref) {
		return true
	}

	// Check properties
	if schema.Properties != nil {
		for _, prop := range schema.Properties {
			if hasExternalRefsInSchema(prop) {
				return true
			}
		}
	}

	// Check items
	if schema.Items != nil && hasExternalRefsInSchema(schema.Items) {
		return true
	}

	// Check allOf, oneOf, anyOf schemas
	if schema.AllOf != nil {
		for _, s := range schema.AllOf {
			if hasExternalRefsInSchema(s) {
				return true
			}
		}
	}

	return false
}

func hasExternalRefsInPathItem(pathItem *PathItem) bool {
	if pathItem == nil {
		return false
	}

	// Check parameters
	if pathItem.Parameters != nil {
		for _, param := range pathItem.Parameters {
			if param.Ref != nil && isExternalRef(*param.Ref) {
				return true
			}
			if param.Schema != nil && hasExternalRefsInSchema(param.Schema) {
				return true
			}
		}
	}

	// Check operations
	for _, operation := range pathItem.Operations() {
		if operation == nil {
			continue
		}

		// Check operation ref
		if operation.Ref != nil && isExternalRef(*operation.Ref) {
			return true
		}

		// Check parameters
		if operation.Parameters != nil {
			for _, param := range operation.Parameters {
				if param.Ref != nil && isExternalRef(*param.Ref) {
					return true
				}
				if param.Schema != nil && hasExternalRefsInSchema(param.Schema) {
					return true
				}
			}
		}

		// Check request body
		if operation.RequestBody != nil {
			if operation.RequestBody.Ref != nil && isExternalRef(*operation.RequestBody.Ref) {
				return true
			}
			if operation.RequestBody.Content != nil {
				for _, content := range operation.RequestBody.Content {
					if content != nil && content.Schema != nil && hasExternalRefsInSchema(content.Schema) {
						return true
					}
				}
			}
		}

		// Check responses
		if operation.Responses != nil {
			for _, response := range operation.Responses {
				if response.Ref != nil && isExternalRef(*response.Ref) {
					return true
				}
				if response.Content != nil {
					for _, content := range response.Content {
						if content != nil && content.Schema != nil && hasExternalRefsInSchema(content.Schema) {
							return true
						}
					}
				}
			}
		}
	}

	return false
}
