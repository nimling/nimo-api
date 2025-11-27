package merger

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

type MergeOptions struct {
	Title        string
	Description  string
	Version      string
	ContactName  string
	ContactEmail string
	Force        bool
	OutputFormat string
}

func MergeSpecs(specPaths []string, outputPath string, opts MergeOptions) error {
	if len(specPaths) == 0 {
		return fmt.Errorf("no spec files provided")
	}

	if !opts.Force {
		if _, err := os.Stat(outputPath); err == nil {
			return fmt.Errorf("output file %s already exists (use --force to overwrite)", outputPath)
		}
	}

	specs := make([]*openapi3.T, 0, len(specPaths))
	for _, path := range specPaths {
		spec, err := loadSpec(path)
		if err != nil {
			return fmt.Errorf("failed to load %s: %w", path, err)
		}
		specs = append(specs, spec)
	}

	merged := mergeOpenAPISpecs(specs, opts)

	if err := writeSpec(merged, outputPath, opts.OutputFormat); err != nil {
		return fmt.Errorf("failed to write merged spec: %w", err)
	}

	fmt.Printf("✓ Merged %d specs into %s\n", len(specPaths), outputPath)
	if merged.Paths != nil {
		fmt.Printf("  Paths: %d\n", len(merged.Paths.Map()))
	}
	if merged.Components != nil && merged.Components.Schemas != nil {
		fmt.Printf("  Schemas: %d\n", len(merged.Components.Schemas))
	}
	if merged.Tags != nil {
		fmt.Printf("  Tags: %d\n", len(merged.Tags))
	}

	return nil
}

func loadSpec(path string) (*openapi3.T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromData(data)
	if err != nil {
		return nil, err
	}

	return spec, nil
}

func mergeOpenAPISpecs(specs []*openapi3.T, opts MergeOptions) *openapi3.T {
	if len(specs) == 0 {
		return nil
	}

	base := specs[0]
	merged := &openapi3.T{
		OpenAPI: base.OpenAPI,
		Info: &openapi3.Info{
			Title:       getOrDefault(opts.Title, base.Info.Title),
			Description: getOrDefault(opts.Description, base.Info.Description),
			Version:     getOrDefault(opts.Version, base.Info.Version),
		},
		Servers:    base.Servers,
		Paths:      openapi3.NewPaths(),
		Components: &openapi3.Components{},
		Tags:       openapi3.Tags{},
		Security:   base.Security,
	}

	if opts.ContactName != "" || opts.ContactEmail != "" {
		merged.Info.Contact = &openapi3.Contact{
			Name:  opts.ContactName,
			Email: opts.ContactEmail,
		}
	} else if base.Info.Contact != nil {
		merged.Info.Contact = base.Info.Contact
	}

	if merged.Components.Schemas == nil {
		merged.Components.Schemas = make(map[string]*openapi3.SchemaRef)
	}
	if merged.Components.SecuritySchemes == nil {
		merged.Components.SecuritySchemes = make(openapi3.SecuritySchemes)
	}

	tagMap := make(map[string]*openapi3.Tag)

	for _, spec := range specs {
		if spec.Paths != nil {
			for path, pathItem := range spec.Paths.Map() {
				merged.Paths.Set(path, pathItem)
			}
		}

		if spec.Components != nil {
			if spec.Components.Schemas != nil {
				for name, schema := range spec.Components.Schemas {
					merged.Components.Schemas[name] = schema
				}
			}

			if spec.Components.SecuritySchemes != nil {
				for name, scheme := range spec.Components.SecuritySchemes {
					merged.Components.SecuritySchemes[name] = scheme
				}
			}
		}

		if spec.Tags != nil {
			for _, tag := range spec.Tags {
				if tag.Name != "" {
					tagMap[tag.Name] = tag
				}
			}
		}
	}

	for _, tag := range tagMap {
		merged.Tags = append(merged.Tags, tag)
	}

	return merged
}

func writeSpec(spec *openapi3.T, outputPath, format string) error {
	var data []byte
	var err error

	switch format {
	case "json":
		data, err = json.MarshalIndent(spec, "", "  ")
	case "yaml":
		data, err = yaml.Marshal(spec)
	default:
		return fmt.Errorf("unsupported format: %s (use 'yaml' or 'json')", format)
	}

	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, data, 0644)
}

func getOrDefault(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}
