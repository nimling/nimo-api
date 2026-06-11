package internal

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/nimling/nimo-api/converter"
	"github.com/nimling/nimo-api/pkg/ai"
	"github.com/nimling/nimo-api/pkg/parser"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var (
	mainPath      string
	readmePath    string
	aiEndpoint    string
	maxConcurrent int
	outputFile    string
	outputFormat  string
)

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func NewGenerateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate OpenAPI specification from Go Echo handlers",
		Long: `Generate OpenAPI specification by analyzing Go Echo handler code using AI.

This command parses your Go source code to extract API handlers and uses AI
(via Ollama) to generate comprehensive OpenAPI documentation including paths,
schemas, and security definitions.

Environment Variables:
  AI_ENDPOINT       - Default AI API endpoint (default: http://localhost:11434)
  MAX_CONCURRENT    - Default max concurrent AI calls (default: 5)
  OUTPUT_FILE       - Default output file path (default: openapi.yaml)
  OUTPUT_FORMAT     - Default output format: yaml or json (default: yaml)

Examples:
  # Generate spec from Go handlers
  nimo generate -m ./main.go -r ./README.md

  # Specify AI endpoint and output
  nimo generate -m ./main.go -r ./README.md -a http://localhost:11434 -o api.yaml

  # Use environment variables
  export AI_ENDPOINT=http://localhost:11434
  export OUTPUT_FILE=openapi.json
  nimo generate -m ./main.go -r ./README.md -f json`,
		RunE: runGenerateCommand,
	}

	cmd.Flags().StringVarP(&mainPath, "main", "m", "", "Path to main.go file (required)")
	cmd.Flags().StringVarP(&readmePath, "readme", "r", "", "Path to README.md file (required)")
	cmd.Flags().StringVarP(&aiEndpoint, "ai-endpoint", "a", getEnvOrDefault("AI_ENDPOINT", "http://localhost:11434"), "AI API endpoint")
	cmd.Flags().IntVarP(&maxConcurrent, "max-concurrent", "c", getEnvInt("MAX_CONCURRENT", 5), "Maximum concurrent AI API calls")
	cmd.Flags().StringVarP(&outputFile, "output", "o", getEnvOrDefault("NIMO_OUTPUT", getEnvOrDefault("OUTPUT_FILE", "openapi.yaml")), "Output file path")
	cmd.Flags().StringVarP(&outputFormat, "format", "f", getEnvOrDefault("OUTPUT_FORMAT", "yaml"), "Output format (yaml or json)")

	cmd.MarkFlagRequired("main")
	cmd.MarkFlagRequired("readme")

	return cmd
}

func runGenerateCommand(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal. Shutting down...")
		cancel()
	}()

	aiClient := ai.NewClient(aiEndpoint, maxConcurrent)

	content, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("error reading README: %w", err)
	}

	spec := initializeSpec(content)
	handlers := parser.ParseHandlers(mainPath)

	var wg sync.WaitGroup
	results := make(chan parser.HandlerResult, len(handlers))

	errCount := 0
	for path, handler := range handlers {
		select {
		case <-ctx.Done():
			fmt.Println("Processing cancelled")
			return ctx.Err()
		default:
			wg.Add(1)
			go func(p string, h parser.Handler) {
				defer wg.Done()
				parser.ProcessHandler(p, h, spec.Components, aiClient, results)
			}(path, handler)
		}
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		if result.Err != nil {
			errCount++
			fmt.Printf("Error processing %s: %v\n", result.Path, result.Err)
			continue
		}

		if spec.Paths == nil {
			spec.Paths = &openapi3.Paths{}
		}
		spec.Paths.Set(result.Path, result.PathDef)
	}

	if errCount > 0 {
		fmt.Printf("Completed with %d errors\n", errCount)
	}

	if err := writeSpec(spec, outputFile, outputFormat); err != nil {
		return fmt.Errorf("error writing spec: %w", err)
	}

	fmt.Printf("Successfully generated OpenAPI specification: %s\n", outputFile)
	return nil
}

func initializeSpec(readmeContent []byte) *openapi3.T {
	title := "API Documentation"
	description := string(readmeContent)

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

	return spec
}

func writeSpec(spec *openapi3.T, outputPath, format string) error {
	var data []byte
	var err error

	switch format {
	case "json":
		data, err = spec.MarshalJSON()
	case "yaml":
		data, err = yaml.Marshal(spec)
	default:
		return fmt.Errorf("unsupported format: %s (use 'yaml' or 'json')", format)
	}

	if err != nil {
		return fmt.Errorf("error marshaling spec: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("error writing spec file: %w", err)
	}
	converter.LogWrite(outputPath, len(data))

	return nil
}
