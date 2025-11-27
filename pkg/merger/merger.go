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

	merged, overwrites := mergeOpenAPISpecs(specs, opts)

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

	if len(overwrites) > 0 {
		fmt.Printf("\n⚠ Warning: %d operation(s) were overwritten:\n", len(overwrites))
		for _, overwrite := range overwrites {
			fmt.Printf("  - %s\n", overwrite)
		}
		fmt.Println("\nPlease review these manually to ensure the correct operations were kept.")
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

func mergeOpenAPISpecs(specs []*openapi3.T, opts MergeOptions) (*openapi3.T, []string) {
	if len(specs) == 0 {
		return nil, nil
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
	var overwrites []string

	for _, spec := range specs {
		if spec.Paths != nil {
			for path, pathItem := range spec.Paths.Map() {
				existingPathItem := merged.Paths.Find(path)
				if existingPathItem == nil {
					merged.Paths.Set(path, pathItem)
				} else {
					overwritten := mergePathItems(existingPathItem, pathItem, path)
					overwrites = append(overwrites, overwritten...)
				}
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

	return merged, overwrites
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

func mergePathItems(existing, new *openapi3.PathItem, path string) []string {
	var overwrites []string

	if new.Get != nil {
		if existing.Get != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s GET", path))
		}
		existing.Get = new.Get
	}
	if new.Post != nil {
		if existing.Post != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s POST", path))
		}
		existing.Post = new.Post
	}
	if new.Put != nil {
		if existing.Put != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s PUT", path))
		}
		existing.Put = new.Put
	}
	if new.Patch != nil {
		if existing.Patch != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s PATCH", path))
		}
		existing.Patch = new.Patch
	}
	if new.Delete != nil {
		if existing.Delete != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s DELETE", path))
		}
		existing.Delete = new.Delete
	}
	if new.Head != nil {
		if existing.Head != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s HEAD", path))
		}
		existing.Head = new.Head
	}
	if new.Options != nil {
		if existing.Options != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s OPTIONS", path))
		}
		existing.Options = new.Options
	}
	if new.Trace != nil {
		if existing.Trace != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s TRACE", path))
		}
		existing.Trace = new.Trace
	}
	if new.Connect != nil {
		if existing.Connect != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s CONNECT", path))
		}
		existing.Connect = new.Connect
	}
	if new.Servers != nil {
		existing.Servers = new.Servers
	}
	if new.Parameters != nil {
		existing.Parameters = new.Parameters
	}

	return overwrites
}
