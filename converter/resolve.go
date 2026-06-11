package converter

import (
	"fmt"
	"github.com/nimling/nimo-api/utils"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

// Verbose toggles diagnostic logging during ref resolution and inlining.
var Verbose bool

// VerboseWrite toggles a bullet line per file write performed by the converter.
var VerboseWrite bool

// LogWrite prints a bullet line for a file write when VerboseWrite is on.
func LogWrite(path string, bytes int) {
	if !VerboseWrite {
		return
	}
	fmt.Fprintf(os.Stderr, "  - wrote %s (%d bytes)\n", path, bytes)
}

func (n *OpenAPIConverter) ResolveExternalRefs() error {
	// Initialize component register if not already done
	if n.doc.Components == nil {
		n.doc.Components = &Components{
			SecuritySchemes: map[string]*SecurityScheme{},
			Parameters:      map[string]*Parameter{},
			Schemas:         map[string]*Schema{},
			Examples:        map[string]*Example{},
			Register:        ReferenceRegister{},
		}
	}
	if n.doc.Components.Examples == nil {
		n.doc.Components.Examples = map[string]*Example{}
	}

	// Iteratively resolve references until no external refs remain
	maxIterations := 20 // Prevent infinite loops
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

		if Verbose {
			refs := n.collectExternalRefs()
			fmt.Fprintf(os.Stderr, "[nimo] iteration %d: %d external refs remain\n", i+1, len(refs))
			sample := refs
			if len(sample) > 10 {
				sample = sample[:10]
			}
			for _, r := range sample {
				fmt.Fprintf(os.Stderr, "[nimo]   - %s\n", r)
			}
			if len(refs) > len(sample) {
				fmt.Fprintf(os.Stderr, "[nimo]   ... and %d more\n", len(refs)-len(sample))
			}
		}

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

			// Process request body content schemas and examples
			if op.RequestBody.Content != nil {
				for mediaType, content := range op.RequestBody.Content {
					if content == nil {
						continue
					}

					if content.Schema != nil {
						if err := content.Schema.resolveExternalRefs(n.doc.Components, relPath); err != nil {
							return fmt.Errorf("failed to resolve request body schema for %s: %w", mediaType, err)
						}
					}

					if err := n.resolveContentExamples(content, relPath); err != nil {
						return fmt.Errorf("failed to resolve request body examples for %s: %w", mediaType, err)
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

				// Process response content schemas and examples
				if response.Content != nil {
					for mediaType, content := range response.Content {
						if content == nil {
							continue
						}

						if content.Schema != nil {
							if err := content.Schema.resolveExternalRefs(n.doc.Components, relPath); err != nil {
								return fmt.Errorf("failed to resolve response schema for %s: %w", mediaType, err)
							}
						}

						if err := n.resolveContentExamples(content, relPath); err != nil {
							return fmt.Errorf("failed to resolve response examples for %s: %w", mediaType, err)
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

	// Handle oneOf array - external refs nested under oneOf must also hoist
	// into components.schemas so consumers do not see raw file refs.
	if r.OneOf != nil {
		for i, schema := range r.OneOf {
			if schema == nil {
				continue
			}
			if err := schema.resolveExternalRefs(components, relPath); err != nil {
				return fmt.Errorf("failed to resolve oneOf[%d]: %w", i, err)
			}
		}
	}

	// Handle anyOf array - same treatment as oneOf.
	if r.AnyOf != nil {
		for i, schema := range r.AnyOf {
			if schema == nil {
				continue
			}
			if err := schema.resolveExternalRefs(components, relPath); err != nil {
				return fmt.Errorf("failed to resolve anyOf[%d]: %w", i, err)
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
			if err := n.resolveParameterExamples(param, relPath); err != nil {
				return nil, fmt.Errorf("failed to resolve parameter examples %s: %w", param.Name, err)
			}
			if param.Schema != nil {
				if err := param.Schema.resolveExternalRefs(n.doc.Components, relPath); err != nil {
					return nil, fmt.Errorf("failed to resolve parameter schema refs %s: %w", param.Name, err)
				}
			}
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
					return fmt.Errorf("failed to load external securityScheme ref %s: %w", filePath, err)
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
					return fmt.Errorf("failed to load external parameter ref %s: %w", filePath, err)
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
					return fmt.Errorf("failed to load external schema ref %s: %w", relPath, err)
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

	if components.Examples != nil {
		for key, ex := range components.Examples {
			if ex == nil {
				continue
			}
			if ex.Ref != nil && isExternalRef(*ex.Ref) {
				filePath := resolveRefPath(n.filePath, *ex.Ref)
				res, err := loadExampleRef(filePath)
				if err != nil {
					return fmt.Errorf("failed to load external example ref %s: %w", filePath, err)
				}
				components.PutRegister("examples", filePath)
				components.Examples[key] = res
			}
		}
	}

	return nil
}

func resolveRefPath(specPath, refPath string) string {
	filePath, fragment := splitRefPath(refPath)

	resolved := filePath
	if !filepath.IsAbs(filePath) {
		baseDir := filepath.Dir(specPath)
		resolved = filepath.Join(baseDir, filePath)
	}
	resolved = filepath.Clean(resolved)

	if fragment != "" {
		return resolved + "#" + fragment
	}
	return resolved
}

func loadExternalRef[T any](filePath string) (*T, error) {
	rawPath, fragment := splitRefPath(filePath)

	content, err := os.ReadFile(rawPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", rawPath, err)
	}

	if fragment == "" {
		var result T
		if err = yaml.Unmarshal(content, &result); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", rawPath, err)
		}
		return &result, nil
	}

	var root any
	if err = yaml.Unmarshal(content, &root); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", rawPath, err)
	}

	sub, err := resolveJSONPointer(root, fragment)
	if err != nil {
		return nil, fmt.Errorf("failed to navigate fragment %q in %s: %w", fragment, rawPath, err)
	}

	subBytes, err := yaml.Marshal(sub)
	if err != nil {
		return nil, fmt.Errorf("failed to re-encode fragment of %s: %w", rawPath, err)
	}

	var result T
	if err = yaml.Unmarshal(subBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to decode fragment of %s into target type: %w", rawPath, err)
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
	if refPath == "" || strings.HasPrefix(refPath, "#") {
		return false
	}
	if strings.Contains(refPath, "://") {
		return true
	}
	if strings.HasPrefix(refPath, "./") ||
		strings.HasPrefix(refPath, "../") ||
		strings.HasPrefix(refPath, "/") {
		return true
	}
	if len(refPath) >= 2 && refPath[1] == ':' {
		return true
	}
	filePart, _ := splitRefPath(refPath)
	if filePart == "" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(filePart))
	return ext == ".yml" || ext == ".yaml" || ext == ".json"
}

func resolveJSONPointer(doc any, fragment string) (any, error) {
	pointer := strings.TrimPrefix(fragment, "/")
	if pointer == "" {
		return doc, nil
	}
	current := doc
	for _, raw := range strings.Split(pointer, "/") {
		token := strings.NewReplacer("~1", "/", "~0", "~").Replace(raw)
		switch v := current.(type) {
		case map[string]any:
			next, ok := v[token]
			if !ok {
				return nil, fmt.Errorf("key %q not found", token)
			}
			current = next
		case map[any]any:
			next, ok := v[token]
			if !ok {
				return nil, fmt.Errorf("key %q not found", token)
			}
			current = next
		case []any:
			idx := -1
			if _, err := fmt.Sscanf(token, "%d", &idx); err != nil || idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("array index %q out of range", token)
			}
			current = v[idx]
		default:
			return nil, fmt.Errorf("cannot descend into %T at token %q", current, token)
		}
	}
	return current, nil
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

// collectExternalRefs walks the doc and returns every external ref string that
// is still present. Used by the verbose ResolveExternalRefs loop to surface
// which refs are not being resolved across iterations.
func (n *OpenAPIConverter) collectExternalRefs() []string {
	var out []string
	if n.doc.Components != nil {
		collectExternalRefsInComponents(n.doc.Components, &out)
	}
	if n.doc.Paths != nil {
		for key, pathItem := range n.doc.Paths {
			collectExternalRefsInPathItem(pathItem, key, &out)
		}
	}
	return out
}

func collectExternalRefsInComponents(components *Components, out *[]string) {
	if components == nil {
		return
	}
	if components.Schemas != nil {
		for name, schema := range components.Schemas {
			collectExternalRefsInSchema(schema, "components.schemas."+name, out)
		}
	}
	if components.Parameters != nil {
		for name, param := range components.Parameters {
			if param.Ref != nil && isExternalRef(*param.Ref) {
				*out = append(*out, "components.parameters."+name+" -> "+*param.Ref)
			}
			collectExternalRefsInSchema(param.Schema, "components.parameters."+name+".schema", out)
		}
	}
	if components.SecuritySchemes != nil {
		for name, scheme := range components.SecuritySchemes {
			if scheme.Ref != nil && isExternalRef(*scheme.Ref) {
				*out = append(*out, "components.securitySchemes."+name+" -> "+*scheme.Ref)
			}
		}
	}
	for name, ex := range components.Examples {
		if ex != nil && ex.Ref != nil && isExternalRef(*ex.Ref) {
			*out = append(*out, "components.examples."+name+" -> "+*ex.Ref)
		}
	}
}

func collectExternalRefsInSchema(schema *Schema, path string, out *[]string) {
	if schema == nil {
		return
	}
	if schema.Ref != nil && isExternalRef(*schema.Ref) {
		*out = append(*out, path+" -> "+*schema.Ref)
	}
	for name, prop := range schema.Properties {
		collectExternalRefsInSchema(prop, path+".properties."+name, out)
	}
	if schema.Items != nil {
		collectExternalRefsInSchema(schema.Items, path+".items", out)
	}
	for i, s := range schema.AllOf {
		collectExternalRefsInSchema(s, fmt.Sprintf("%s.allOf[%d]", path, i), out)
	}
	for i, s := range schema.OneOf {
		collectExternalRefsInSchema(s, fmt.Sprintf("%s.oneOf[%d]", path, i), out)
	}
	for i, s := range schema.AnyOf {
		collectExternalRefsInSchema(s, fmt.Sprintf("%s.anyOf[%d]", path, i), out)
	}
}

func collectExternalRefsInPathItem(pathItem *PathItem, key string, out *[]string) {
	if pathItem == nil {
		return
	}
	for i, param := range pathItem.Parameters {
		base := fmt.Sprintf("paths[%s].parameters[%d]", key, i)
		if param.Ref != nil && isExternalRef(*param.Ref) {
			*out = append(*out, base+" -> "+*param.Ref)
		}
		collectExternalRefsInSchema(param.Schema, base+".schema", out)
	}
	for opName, operation := range pathItem.Operations() {
		if operation == nil {
			continue
		}
		base := fmt.Sprintf("paths[%s].%s", key, opName)
		if operation.Ref != nil && isExternalRef(*operation.Ref) {
			*out = append(*out, base+" -> "+*operation.Ref)
		}
		for i, param := range operation.Parameters {
			pbase := fmt.Sprintf("%s.parameters[%d]", base, i)
			if param.Ref != nil && isExternalRef(*param.Ref) {
				*out = append(*out, pbase+" -> "+*param.Ref)
			}
			collectExternalRefsInSchema(param.Schema, pbase+".schema", out)
		}
		if operation.RequestBody != nil {
			rb := operation.RequestBody
			if rb.Ref != nil && isExternalRef(*rb.Ref) {
				*out = append(*out, base+".requestBody -> "+*rb.Ref)
			}
			for mt, content := range rb.Content {
				if content == nil {
					continue
				}
				collectExternalRefsInSchema(content.Schema, base+".requestBody.content["+mt+"].schema", out)
			}
		}
		for code, response := range operation.Responses {
			rbase := base + ".responses[" + code + "]"
			if response.Ref != nil && isExternalRef(*response.Ref) {
				*out = append(*out, rbase+" -> "+*response.Ref)
			}
			for mt, content := range response.Content {
				if content == nil {
					continue
				}
				collectExternalRefsInSchema(content.Schema, rbase+".content["+mt+"].schema", out)
			}
		}
	}
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
			if hasExternalRefsInExamples(param.Examples) {
				return true
			}
			if singularExampleHasExternalRef(param.Example) {
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

	if hasExternalRefsInExamples(components.Examples) {
		return true
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
	if schema.OneOf != nil {
		for _, s := range schema.OneOf {
			if hasExternalRefsInSchema(s) {
				return true
			}
		}
	}
	if schema.AnyOf != nil {
		for _, s := range schema.AnyOf {
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
				if hasExternalRefsInExamples(param.Examples) {
					return true
				}
				if singularExampleHasExternalRef(param.Example) {
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
					if content == nil {
						continue
					}
					if content.Schema != nil && hasExternalRefsInSchema(content.Schema) {
						return true
					}
					if hasExternalRefsInExamples(content.Examples) {
						return true
					}
					if singularExampleHasExternalRef(content.Example) {
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
						if content == nil {
							continue
						}
						if content.Schema != nil && hasExternalRefsInSchema(content.Schema) {
							return true
						}
						if hasExternalRefsInExamples(content.Examples) {
							return true
						}
						if singularExampleHasExternalRef(content.Example) {
							return true
						}
					}
				}
			}
		}
	}

	return false
}

func hasExternalRefsInExamples(examples map[string]*Example) bool {
	for _, ex := range examples {
		if ex == nil {
			continue
		}
		if ex.Ref != nil && isExternalRef(*ex.Ref) {
			return true
		}
	}
	return false
}

func singularExampleHasExternalRef(ex interface{}) bool {
	ref, ok := extractSingleRef(ex)
	if !ok {
		return false
	}
	return isExternalRef(ref)
}

func extractSingleRef(v interface{}) (string, bool) {
	switch m := v.(type) {
	case map[string]interface{}:
		if len(m) != 1 {
			return "", false
		}
		raw, ok := m["$ref"]
		if !ok {
			return "", false
		}
		s, ok := raw.(string)
		return s, ok
	case map[interface{}]interface{}:
		if len(m) != 1 {
			return "", false
		}
		raw, ok := m["$ref"]
		if !ok {
			return "", false
		}
		s, ok := raw.(string)
		return s, ok
	}
	return "", false
}

func (n *OpenAPIConverter) resolveContentExamples(content *ResponseContent, relPath string) error {
	if content == nil {
		return nil
	}
	if err := n.resolveSingularExample(&content.Example, relPath); err != nil {
		return err
	}
	return n.resolveExamplesMap(content.Examples, relPath)
}

func (n *OpenAPIConverter) resolveParameterExamples(param *Parameter, relPath string) error {
	if param == nil {
		return nil
	}
	if err := n.resolveSingularExample(&param.Example, relPath); err != nil {
		return err
	}
	return n.resolveExamplesMap(param.Examples, relPath)
}

func (n *OpenAPIConverter) resolveSingularExample(ex *interface{}, relPath string) error {
	if ex == nil || *ex == nil {
		return nil
	}
	ref, ok := extractSingleRef(*ex)
	if !ok || !isExternalRef(ref) {
		return nil
	}
	filePath := resolveRefPath(relPath, ref)
	loaded, err := loadExampleRef(filePath)
	if err != nil {
		return fmt.Errorf("failed to load singular example ref %s: %w", ref, err)
	}
	*ex = loaded.Value
	return nil
}

func (n *OpenAPIConverter) resolveExamplesMap(examples map[string]*Example, relPath string) error {
	if examples == nil {
		return nil
	}
	for name, ex := range examples {
		if ex == nil || ex.Ref == nil || !isExternalRef(*ex.Ref) {
			continue
		}
		filePath := resolveRefPath(relPath, *ex.Ref)

		if existingRef, exists := n.doc.Components.Register[filePath]; exists {
			examples[name] = &Example{Ref: utils.StringPtr(existingRef)}
			continue
		}

		loaded, err := loadExampleRef(filePath)
		if err != nil {
			return fmt.Errorf("failed to load example ref %s: %w", *ex.Ref, err)
		}

		comp := n.doc.Components.PutRegister("examples", filePath)
		if n.doc.Components.Examples == nil {
			n.doc.Components.Examples = map[string]*Example{}
		}
		if n.doc.Components.Examples[comp.Name] == nil {
			n.doc.Components.Examples[comp.Name] = loaded
		}
		examples[name] = &Example{Ref: utils.StringPtr(comp.Identifier)}
	}
	return nil
}

func loadExampleRef(filePath string) (*Example, error) {
	rawPath, fragment := splitRefPath(filePath)

	content, err := os.ReadFile(rawPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", rawPath, err)
	}

	var root any
	if err = yaml.Unmarshal(content, &root); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", rawPath, err)
	}

	if fragment != "" {
		root, err = resolveJSONPointer(root, fragment)
		if err != nil {
			return nil, fmt.Errorf("failed to navigate fragment %q in %s: %w", fragment, rawPath, err)
		}
	}

	if isExampleObjectShape(root) {
		bytes, err := yaml.Marshal(root)
		if err != nil {
			return nil, fmt.Errorf("failed to re-encode example %s: %w", rawPath, err)
		}
		var ex Example
		if err = yaml.Unmarshal(bytes, &ex); err != nil {
			return nil, fmt.Errorf("failed to decode example %s: %w", rawPath, err)
		}
		return &ex, nil
	}

	return &Example{Value: normalizeYAML(root)}, nil
}

func isExampleObjectShape(v any) bool {
	keys := map[string]struct{}{
		"$ref": {}, "summary": {}, "description": {}, "value": {}, "externalValue": {},
	}
	switch m := v.(type) {
	case map[string]any:
		if len(m) == 0 {
			return false
		}
		for k := range m {
			if _, ok := keys[k]; !ok {
				return false
			}
		}
		return true
	case map[any]any:
		if len(m) == 0 {
			return false
		}
		for k := range m {
			s, ok := k.(string)
			if !ok {
				return false
			}
			if _, ok := keys[s]; !ok {
				return false
			}
		}
		return true
	}
	return false
}

func normalizeYAML(v any) any {
	switch m := v.(type) {
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			s, ok := k.(string)
			if !ok {
				s = fmt.Sprintf("%v", k)
			}
			out[s] = normalizeYAML(val)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[k] = normalizeYAML(val)
		}
		return out
	case []any:
		out := make([]any, len(m))
		for i, val := range m {
			out[i] = normalizeYAML(val)
		}
		return out
	}
	return v
}
