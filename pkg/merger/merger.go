package merger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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
	Strategy     string
	ServerURL    string
	TextFormat   string
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

	merged, report := mergeOpenAPISpecs(specs, opts)

	if opts.TextFormat != "asis" {
		convertDescriptions(merged, opts.TextFormat)
	}

	if err := writeSpec(merged, outputPath, opts.OutputFormat); err != nil {
		return fmt.Errorf("failed to write merged spec: %w", err)
	}

	fmt.Printf("✓ Merged %d specs into %s\n", len(specPaths), outputPath)
	if merged.Paths != nil {
		fmt.Printf("  Paths: %d\n", len(merged.Paths.Map()))
	}
	if merged.Components != nil {
		if merged.Components.Schemas != nil {
			fmt.Printf("  Schemas: %d\n", len(merged.Components.Schemas))
		}
		if merged.Components.Responses != nil {
			fmt.Printf("  Responses: %d\n", len(merged.Components.Responses))
		}
		if merged.Components.Parameters != nil {
			fmt.Printf("  Parameters: %d\n", len(merged.Components.Parameters))
		}
	}
	if merged.Tags != nil {
		fmt.Printf("  Tags: %d\n", len(merged.Tags))
	}

	report.Print()

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

type MergeReport struct {
	PathConflicts      []string
	ComponentConflicts []string
	MissingRefs        []string
}

func (r *MergeReport) Print() {
	if len(r.PathConflicts) > 0 {
		fmt.Printf("\n⚠ Path Conflicts: %d operation(s) were overwritten:\n", len(r.PathConflicts))
		for _, conflict := range r.PathConflicts {
			fmt.Printf("  - %s\n", conflict)
		}
	}

	if len(r.ComponentConflicts) > 0 {
		fmt.Printf("\n⚠ Component Conflicts: %d component(s) had multiple definitions:\n", len(r.ComponentConflicts))
		for _, conflict := range r.ComponentConflicts {
			fmt.Printf("  - %s\n", conflict)
		}
	}

	if len(r.MissingRefs) > 0 {
		fmt.Printf("\n❌ Missing References: %d unresolved reference(s):\n", len(r.MissingRefs))
		for _, ref := range r.MissingRefs {
			fmt.Printf("  - %s\n", ref)
		}
		fmt.Println("\nThese references point to components that don't exist in any of the source specs.")
	}

	if len(r.PathConflicts) == 0 && len(r.ComponentConflicts) == 0 && len(r.MissingRefs) == 0 {
		fmt.Println("\n✓ No conflicts detected")
	}
}

func mergeOpenAPISpecs(specs []*openapi3.T, opts MergeOptions) (*openapi3.T, *MergeReport) {
	if len(specs) == 0 {
		return nil, &MergeReport{}
	}

	report := &MergeReport{
		PathConflicts:      []string{},
		ComponentConflicts: []string{},
		MissingRefs:        []string{},
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

	if opts.ServerURL != "" {
		merged.Servers = openapi3.Servers{
			{
				URL:         opts.ServerURL,
				Description: "Development",
			},
		}
	}

	if opts.ContactName != "" || opts.ContactEmail != "" {
		merged.Info.Contact = &openapi3.Contact{
			Name:  opts.ContactName,
			Email: opts.ContactEmail,
		}
	} else if base.Info.Contact != nil {
		merged.Info.Contact = base.Info.Contact
	}

	merged.Components.Schemas = make(map[string]*openapi3.SchemaRef)
	merged.Components.SecuritySchemes = make(openapi3.SecuritySchemes)
	merged.Components.Responses = make(openapi3.ResponseBodies)
	merged.Components.Parameters = make(openapi3.ParametersMap)
	merged.Components.RequestBodies = make(openapi3.RequestBodies)
	merged.Components.Headers = make(openapi3.Headers)
	merged.Components.Examples = make(openapi3.Examples)
	merged.Components.Links = make(openapi3.Links)
	merged.Components.Callbacks = make(openapi3.Callbacks)

	tagMap := make(map[string]*openapi3.Tag)

	for specIdx, spec := range specs {
		if spec.Paths != nil {
			for path, pathItem := range spec.Paths.Map() {
				existingPathItem := merged.Paths.Find(path)
				if existingPathItem == nil {
					merged.Paths.Set(path, pathItem)
				} else {
					overwritten := mergePathItems(existingPathItem, pathItem, path, opts.Strategy)
					report.PathConflicts = append(report.PathConflicts, overwritten...)
				}
			}
		}

		if spec.Components != nil {
			mergeComponents(merged.Components, spec.Components, specIdx, opts.Strategy, report)
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

	return merged, report
}

func writeSpec(spec *openapi3.T, outputPath, format string) error {
	var data []byte
	var err error

	switch format {
	case "json":
		buffer := &bytes.Buffer{}
		encoder := json.NewEncoder(buffer)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(spec)
		if err != nil {
			return err
		}
		data = buffer.Bytes()
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

func mergeComponents(target, source *openapi3.Components, specIdx int, strategy string, report *MergeReport) {
	useFirst := strategy == "first"

	if source.Schemas != nil {
		for name, schema := range source.Schemas {
			if _, exists := target.Schemas[name]; exists {
				if !useFirst {
					target.Schemas[name] = schema
					report.ComponentConflicts = append(report.ComponentConflicts, fmt.Sprintf("Schema '%s' (using %s)", name, strategy))
				}
			} else {
				target.Schemas[name] = schema
			}
		}
	}

	if source.Responses != nil {
		for name, response := range source.Responses {
			if _, exists := target.Responses[name]; exists {
				if !useFirst {
					target.Responses[name] = response
					report.ComponentConflicts = append(report.ComponentConflicts, fmt.Sprintf("Response '%s' (using %s)", name, strategy))
				}
			} else {
				target.Responses[name] = response
			}
		}
	}

	if source.Parameters != nil {
		for name, param := range source.Parameters {
			if _, exists := target.Parameters[name]; exists {
				if !useFirst {
					target.Parameters[name] = param
					report.ComponentConflicts = append(report.ComponentConflicts, fmt.Sprintf("Parameter '%s' (using %s)", name, strategy))
				}
			} else {
				target.Parameters[name] = param
			}
		}
	}

	if source.RequestBodies != nil {
		for name, body := range source.RequestBodies {
			if _, exists := target.RequestBodies[name]; exists {
				if !useFirst {
					target.RequestBodies[name] = body
					report.ComponentConflicts = append(report.ComponentConflicts, fmt.Sprintf("RequestBody '%s' (using %s)", name, strategy))
				}
			} else {
				target.RequestBodies[name] = body
			}
		}
	}

	if source.Headers != nil {
		for name, header := range source.Headers {
			if _, exists := target.Headers[name]; !exists || !useFirst {
				target.Headers[name] = header
			}
		}
	}

	if source.SecuritySchemes != nil {
		for name, scheme := range source.SecuritySchemes {
			if _, exists := target.SecuritySchemes[name]; !exists || !useFirst {
				target.SecuritySchemes[name] = scheme
			}
		}
	}

	if source.Examples != nil {
		for name, example := range source.Examples {
			if _, exists := target.Examples[name]; !exists || !useFirst {
				target.Examples[name] = example
			}
		}
	}

	if source.Links != nil {
		for name, link := range source.Links {
			if _, exists := target.Links[name]; !exists || !useFirst {
				target.Links[name] = link
			}
		}
	}

	if source.Callbacks != nil {
		for name, callback := range source.Callbacks {
			if _, exists := target.Callbacks[name]; !exists || !useFirst {
				target.Callbacks[name] = callback
			}
		}
	}
}

func mergePathItems(existing, new *openapi3.PathItem, path string, strategy string) []string {
	var overwrites []string
	useFirst := strategy == "first"

	if new.Get != nil {
		if existing.Get != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s GET (using %s)", path, strategy))
			if !useFirst {
				existing.Get = new.Get
			}
		} else {
			existing.Get = new.Get
		}
	}
	if new.Post != nil {
		if existing.Post != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s POST (using %s)", path, strategy))
			if !useFirst {
				existing.Post = new.Post
			}
		} else {
			existing.Post = new.Post
		}
	}
	if new.Put != nil {
		if existing.Put != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s PUT (using %s)", path, strategy))
			if !useFirst {
				existing.Put = new.Put
			}
		} else {
			existing.Put = new.Put
		}
	}
	if new.Patch != nil {
		if existing.Patch != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s PATCH (using %s)", path, strategy))
			if !useFirst {
				existing.Patch = new.Patch
			}
		} else {
			existing.Patch = new.Patch
		}
	}
	if new.Delete != nil {
		if existing.Delete != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s DELETE (using %s)", path, strategy))
			if !useFirst {
				existing.Delete = new.Delete
			}
		} else {
			existing.Delete = new.Delete
		}
	}
	if new.Head != nil {
		if existing.Head != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s HEAD (using %s)", path, strategy))
			if !useFirst {
				existing.Head = new.Head
			}
		} else {
			existing.Head = new.Head
		}
	}
	if new.Options != nil {
		if existing.Options != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s OPTIONS (using %s)", path, strategy))
			if !useFirst {
				existing.Options = new.Options
			}
		} else {
			existing.Options = new.Options
		}
	}
	if new.Trace != nil {
		if existing.Trace != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s TRACE (using %s)", path, strategy))
			if !useFirst {
				existing.Trace = new.Trace
			}
		} else {
			existing.Trace = new.Trace
		}
	}
	if new.Connect != nil {
		if existing.Connect != nil {
			overwrites = append(overwrites, fmt.Sprintf("%s CONNECT (using %s)", path, strategy))
			if !useFirst {
				existing.Connect = new.Connect
			}
		} else {
			existing.Connect = new.Connect
		}
	}
	if new.Servers != nil && !useFirst {
		existing.Servers = new.Servers
	}
	if new.Parameters != nil && !useFirst {
		existing.Parameters = new.Parameters
	}

	return overwrites
}

func convertDescriptions(spec *openapi3.T, format string) {
	if spec.Info != nil && spec.Info.Description != "" {
		spec.Info.Description = convertText(spec.Info.Description, format)
	}

	if spec.Paths != nil {
		for _, pathItem := range spec.Paths.Map() {
			convertOperationDescriptions(pathItem.Get, format)
			convertOperationDescriptions(pathItem.Post, format)
			convertOperationDescriptions(pathItem.Put, format)
			convertOperationDescriptions(pathItem.Patch, format)
			convertOperationDescriptions(pathItem.Delete, format)
			convertOperationDescriptions(pathItem.Head, format)
			convertOperationDescriptions(pathItem.Options, format)
		}
	}

	if spec.Components != nil {
		if spec.Components.Schemas != nil {
			for _, schema := range spec.Components.Schemas {
				convertSchemaDescriptions(schema.Value, format)
			}
		}
	}

	if spec.Tags != nil {
		for _, tag := range spec.Tags {
			if tag.Description != "" {
				tag.Description = convertText(tag.Description, format)
			}
		}
	}
}

func convertOperationDescriptions(op *openapi3.Operation, format string) {
	if op == nil {
		return
	}
	if op.Description != "" {
		op.Description = convertText(op.Description, format)
	}
	if op.Summary != "" {
		op.Summary = convertText(op.Summary, format)
	}
}

func convertSchemaDescriptions(schema *openapi3.Schema, format string) {
	if schema == nil {
		return
	}
	if schema.Description != "" {
		schema.Description = convertText(schema.Description, format)
	}
	for _, prop := range schema.Properties {
		convertSchemaDescriptions(prop.Value, format)
	}
}

func convertText(text, format string) string {
	switch format {
	case "html":
		return markdownToHTML(text)
	case "markdown":
		return text
	default:
		return text
	}
}

func markdownToHTML(markdown string) string {
	html := markdown

	html = strings.ReplaceAll(html, "\\n\\n", "\n\n")
	html = strings.ReplaceAll(html, "\\n", "\n")

	lines := strings.Split(html, "\n")
	var result []string
	inList := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			if inList {
				result = append(result, "</ul>")
				inList = false
			}
			result = append(result, "<br>")
			continue
		}

		if strings.HasPrefix(trimmed, "### ") {
			if inList {
				result = append(result, "</ul>")
				inList = false
			}
			content := strings.TrimPrefix(trimmed, "### ")
			content = convertInlineMarkdown(content)
			result = append(result, "<h3>"+content+"</h3>")
		} else if strings.HasPrefix(trimmed, "## ") {
			if inList {
				result = append(result, "</ul>")
				inList = false
			}
			content := strings.TrimPrefix(trimmed, "## ")
			content = convertInlineMarkdown(content)
			result = append(result, "<h2>"+content+"</h2>")
		} else if strings.HasPrefix(trimmed, "# ") {
			if inList {
				result = append(result, "</ul>")
				inList = false
			}
			content := strings.TrimPrefix(trimmed, "# ")
			content = convertInlineMarkdown(content)
			result = append(result, "<h1>"+content+"</h1>")
		} else if strings.HasPrefix(trimmed, "- ") {
			if !inList {
				result = append(result, "<ul>")
				inList = true
			}
			content := strings.TrimPrefix(trimmed, "- ")
			content = convertInlineMarkdown(content)
			result = append(result, "<li>"+content+"</li>")
		} else {
			if inList {
				result = append(result, "</ul>")
				inList = false
			}
			result = append(result, convertInlineMarkdown(trimmed))
		}
	}

	if inList {
		result = append(result, "</ul>")
	}

	return strings.Join(result, "\n")
}

func convertInlineMarkdown(text string) string {
	result := text

	for strings.Contains(result, "**") {
		first := strings.Index(result, "**")
		if first == -1 {
			break
		}
		second := strings.Index(result[first+2:], "**")
		if second == -1 {
			break
		}
		second += first + 2

		before := result[:first]
		content := result[first+2 : second]
		after := result[second+2:]
		result = before + "<strong>" + content + "</strong>" + after
	}

	for strings.Contains(result, "*") && !strings.Contains(result, "**") {
		first := strings.Index(result, "*")
		if first == -1 {
			break
		}
		second := strings.Index(result[first+1:], "*")
		if second == -1 {
			break
		}
		second += first + 1

		before := result[:first]
		content := result[first+1 : second]
		after := result[second+1:]
		result = before + "<em>" + content + "</em>" + after
	}

	result = strings.ReplaceAll(result, "`", "<code>")

	return result
}
