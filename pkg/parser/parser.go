package parser

import (
	"context"
	"errors"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/nimling/nimo-api/pkg/ai"
	"gopkg.in/yaml.v2"
	"os"
	"regexp"
	"strings"
)

type Handler struct {
	Code        string
	Middlewares []string
}

type HandlerResult struct {
	Path    string
	PathDef *openapi3.PathItem
	Err     error
}

type Parser struct {
	aiClient   *ai.Client
	readmePath string
}

func NewParser(aiClient *ai.Client, readmePath string) (*Parser, error) {
	if aiClient == nil {
		return nil, errors.New("Missing ai client")
	}

	if readmePath == "" {
		return nil, errors.New("Missing readme path")
	}

	return &Parser{
		aiClient: aiClient,
	}, nil
}

func (p *Parser) InitializeSpec(outputPath string) (*openapi3.T, error) {

	if outputPath == "" {
		return nil, errors.New("Missing output path")
	}

	content, err := os.ReadFile(p.readmePath)
	if err != nil {
		fmt.Printf("Error reading README: %v\n", err)
		os.Exit(1)
	}

	// Extract title from README (first # line)
	title := "API Documentation"
	description := string(content)

	titleRegex := regexp.MustCompile(`(?m)^#\s+(.+)$`)
	if match := titleRegex.FindStringSubmatch(string(content)); len(match) > 1 {
		title = match[1]
	}

	spec := &openapi3.T{
		OpenAPI: "3.1.0",
		Info: &openapi3.Info{
			Title:       title,
			Description: description,
			Version:     "1.0.0",
		},
		Paths: &openapi3.Paths{},
		Components: &openapi3.Components{
			Schemas: make(map[string]*openapi3.SchemaRef),
			SecuritySchemes: openapi3.SecuritySchemes{
				"BearerAuth": &openapi3.SecuritySchemeRef{
					Value: &openapi3.SecurityScheme{
						Type:         "http",
						Scheme:       "bearer",
						BearerFormat: "JWT",
						Description:  "Bearer token authentication",
					},
				},
				"CookieAuth": &openapi3.SecuritySchemeRef{
					Value: &openapi3.SecurityScheme{
						Type:        "apiKey",
						In:          "cookie",
						Name:        "session",
						Description: "Session cookie authentication",
					},
				},
			},
		},
	}

	return spec, nil
}

func (p *Parser) MergePath(spec *openapi3.T, path string, pathDef *openapi3.PathItem) {

	if spec.Paths == nil {
		spec.Paths = &openapi3.Paths{}
	}

	spec.Paths.Set(path, pathDef)
}

func WriteSpec(spec *openapi3.T) error {
	data, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("error marshaling spec: %v", err)
	}

	if err := os.WriteFile("openapi.yaml", data, 0644); err != nil {
		return fmt.Errorf("error writing spec file: %v", err)
	}

	return nil
}

var middlewareSecurityMap = map[string]bool{
	"RequireAuth":     true,
	"RequireAPIKey":   true,
	"AuthMiddleware":  true,
	"SessionRequired": true,
	"ValidateSession": true,
}

func ParseHandlers(mainPath string) map[string]Handler {
	content, err := os.ReadFile(mainPath)
	if err != nil {
		fmt.Printf("Error reading handler file: %v\n", err)
		os.Exit(1)
	}

	handlers := make(map[string]Handler)

	// Extract function content
	funcRegex := regexp.MustCompile(`(?s)func\s+([A-Za-z0-9]+)\s*\([^)]+\)\s*error\s*{([^}]+)}`)
	matches := funcRegex.FindAllStringSubmatch(string(content), -1)

	for _, match := range matches {
		if len(match) >= 3 {
			funcBody := match[2]

			// Look for echo context parameter
			if strings.Contains(funcBody, "echo.Context") {
				// Extract path from function body or comments
				pathRegex := regexp.MustCompile(`c\.Param\("([^"]+)"\)`)
				pathMatches := pathRegex.FindAllStringSubmatch(funcBody, -1)

				path := "/"
				if len(pathMatches) > 0 {
					// Construct path from parameters
					for _, param := range pathMatches {
						path += "{" + param[1] + "}/"
					}
					path = strings.TrimRight(path, "/")
				}

				handlers[path] = Handler{
					Code:        match[0],   // Full function code
					Middlewares: []string{}, // TODO: Extract middleware
				}
			}
		}
	}

	return handlers
}

func ProcessHandler(path string, handler Handler, components *openapi3.Components, client ai.Provider, results chan<- HandlerResult) {
	ctx := context.Background()
	componentsYAML, _ := yaml.Marshal(components)

	resp, err := client.GeneratePathDef(ctx, handler.Code, string(componentsYAML), "")
	if err != nil {
		results <- HandlerResult{Path: path, Err: err}
		return
	}

	pathItem := &openapi3.PathItem{}

	// The Response struct has Paths field that contains the path definitions
	if resp.Paths != nil {
		// For now, create a simple path item as placeholder
		// TODO: Properly unmarshal from resp.Paths
		pathItem = &openapi3.PathItem{}
	}

	// Add security for matched middleware patterns
	for _, middleware := range handler.Middlewares {
		for pattern := range middlewareSecurityMap {
			if matched, _ := regexp.MatchString(pattern, middleware); matched {
				securityReq := openapi3.NewSecurityRequirements()
				securityReq.With(openapi3.NewSecurityRequirement().Authenticate("BearerAuth"))
				securityReq.With(openapi3.NewSecurityRequirement().Authenticate("CookieAuth"))

				if pathItem.Get != nil {
					pathItem.Get.Security = securityReq
				}
				if pathItem.Post != nil {
					pathItem.Post.Security = securityReq
				}
				if pathItem.Put != nil {
					pathItem.Put.Security = securityReq
				}
				if pathItem.Delete != nil {
					pathItem.Delete.Security = securityReq
				}
				if pathItem.Patch != nil {
					pathItem.Patch.Security = securityReq
				}
			}
		}
	}

	results <- HandlerResult{Path: path, PathDef: pathItem}
}
