package converter

import (
	"fmt"
	"github.com/nimling/nimo-api/utils"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

type OpenAPIConverter struct {
	doc               *OpenAPIDoc
	CommonPrefix      string
	filePath          string
	apiTitle          string
	apiDescription    string
	FilePrefix        string
	WriteIntroduction bool
}

// NewOpenApiConverter creates a new OpenApiConverter
func NewOpenApiConverter(filePath string) (*OpenAPIConverter, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	apiDoc := OpenAPIDoc{}
	if err := yaml.Unmarshal(data, &apiDoc); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
	}

	conv := &OpenAPIConverter{
		doc:               &apiDoc,
		filePath:          filePath,
		apiTitle:          apiDoc.Info.Title, //TODO:: Add or show an error here as this can be null
		apiDescription:    apiDoc.Info.Description,
		WriteIntroduction: true,
		FilePrefix:        "",
		CommonPrefix:      "",
	}

	if err = conv.ResolveExternalRefs(); err != nil {
		return nil, fmt.Errorf("error: %s", err)
	}

	return conv, nil
}

func (n *OpenAPIConverter) ValidateDocument() error {
	if n.doc.Info == nil {
		return fmt.Errorf("file '%s': missing required 'info' section in OpenAPIDoc specification", n.filePath)
	}

	if n.doc.Info.Title == "" {
		return fmt.Errorf("file '%s': missing required 'info.title' in OpenAPIDoc specification", n.filePath)
	}

	if n.doc.Info.Description == "" {
		return fmt.Errorf("file '%s': missing required 'info.description' in OpenAPIDoc specification", n.filePath)
	}

	if n.doc.Info.Version == "" {
		return fmt.Errorf("file '%s': missing required 'info.version' in OpenAPIDoc specification", n.filePath)
	}

	if n.doc.Servers == nil || len(n.doc.Servers) <= 0 {
		return fmt.Errorf("file '%s': missing required 'servers' section in OpenAPIDoc specification", n.filePath)
	}

	if n.doc.Servers[0].URL == "" {
		return fmt.Errorf("file '%s': missing required 'servers[0].url' in OpenAPIDoc specification", n.filePath)
	}

	if n.doc.Paths == nil || len(n.doc.Paths) <= 0 {
		return fmt.Errorf("file '%s': no paths found in OpenAPIDoc specification", n.filePath)
	}

	commonPrefix := "/"
	for path := range n.doc.Paths {
		if path == "" {
			return fmt.Errorf("file '%s': empty path found in OpenAPIDoc specification", n.filePath)
		}

		if !strings.HasPrefix(path, "/") {
			return fmt.Errorf("file '%s': path '%s' must start with /", n.filePath, path)
		}

		segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(segments) == 0 {
			return fmt.Errorf("file '%s': path '%s' must have at least one segment", n.filePath, path)
		}

		// Validate path item summary and description
		pathItem, _ := (n.doc.Paths)[path]

		// Check if summary and description exist at path level or in any of the methods
		hasSummaryDesc := (pathItem.Summary != nil && pathItem.Description != nil)

		for _, operation := range pathItem.Operations() {
			if operation != nil {
				// Check for operationId
				if operation.OperationID == nil {
					return fmt.Errorf("file '%s': path '%s' %s operation is missing required 'operationId'",
						n.filePath, path, operation.Method)
				}

				if operation.Summary != nil && operation.Description != nil {
					hasSummaryDesc = true
				}
			}
		}

		if !hasSummaryDesc {
			return fmt.Errorf("file '%s': path '%s' is missing required 'summary' and 'description' fields (must be defined either at path level or in at least one operation)", n.filePath, path)
		}

		// Validate common prefix
		currentPrefix := segments[0]
		if commonPrefix == "/" {
			commonPrefix = currentPrefix
			continue
		} else if commonPrefix != currentPrefix {
			commonPrefix = ""
		}
	}

	if len(n.CommonPrefix) <= 0 {
		n.CommonPrefix = commonPrefix
	}

	return nil
}

func (n *OpenAPIConverter) convertPath(path string, pathItem *PathItem) (string, error) {
	var methods []string
	var summaries []string
	var descriptions []string

	// Collect security requirements for each method
	methodSecurity := make(map[string][]string)

	// Get global security requirements
	globalClaims := make([]string, 0)
	if n.doc.Security != nil && len(*n.doc.Security) > 0 {
		for _, sec := range *n.doc.Security {
			fmt.Println(sec)
			//for _, entraAuth := range sec.Requirements.Value("oauth2") {
			//	globalClaims = append(globalClaims, entraAuth)
			//}
		}
	}

	// Helper function to process operations
	for _, op := range pathItem.Operations() {
		methods = append(methods, op.Method)
		if op.Summary != nil {
			summaries = append(summaries, fmt.Sprintf("%s: %s", op.Method, op.Summary))
		}
		if op.Description != nil {
			descriptions = append(descriptions, fmt.Sprintf("%s: %s", op.Method, op.Description))
		}

		// Check for operation-specific security
		for _, sec := range op.Security {
			fmt.Println(sec)
			//for _, entraAuth := range sec.Requirements.Value("oauth2") {
			//	methodSecurity[method] = append(methodSecurity[method], entraAuth)
			//}
		}
		// If no specific claims but global claims exist, use those
		//if _, hasSpecific := methodSecurity[method]; !hasSpecific && len(globalClaims) > 0 {
		//	methodSecurity[method] = globalClaims
		//}
	}

	data := struct {
		Path         string
		Methods      []string
		AllowMethods string
		ServerURL    string
		Summaries    []string
		Descriptions []string
		Prefix       string
		GlobalClaims []string
		MethodClaims map[string][]string
	}{
		Path:         path,
		Methods:      methods,
		AllowMethods: strings.Join(methods, " "),
		ServerURL:    n.doc.Servers[0].URL,
		Summaries:    summaries,
		Descriptions: descriptions,
		GlobalClaims: globalClaims,
		MethodClaims: methodSecurity,
	}

	if n.CommonPrefix != "" {
		data.Prefix = strings.TrimSuffix(n.CommonPrefix, "/")
	}

	return utils.ExecuteTemplate("nginx", locationTemplate, data)
}
