package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"github.com/nimling/nimo-api/converter"
	"github.com/spf13/cobra"
)

var (
	nginxOutputDir       string
	outputDir            string
	docsDir              string
	generateDocs         bool
	indexPath            string
	filePrefix           string
	commonPrefix         string
	writeIntroduction    bool
	mergeResponsesInline bool
	inlineExamples       bool
	inlineResponses      bool
	inlineSchemas        bool
	flat                 bool
)

func NewConvertCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "convert [files...]",
		Short: "Convert OpenAPI specifications to multiple formats",
		Long: `Convert OpenAPI/Swagger specifications (v3.0+) into various output formats.

The convert command processes YAML/JSON API specifications and generates:
- Nginx location configurations for API gateway routing
- VitePress markdown documentation with interactive API references
- Structured index files for documentation navigation

Examples:
  # Convert a single spec to VitePress docs
  openapi-converter convert api.yml -d ./docs --write-introduction
  
  # Generate Nginx configuration
  openapi-converter convert api.yml -o ./nginx --file-prefix api
  
  # Process multiple specs with common prefix
  openapi-converter convert *.yml -d ./docs --common-prefix v1`,
		Args: cobra.MinimumNArgs(1),
		RunE: runConvertCommand,
	}
	
	cmd.Flags().StringVar(&nginxOutputDir, "nginx-output", "", "Output directory for Nginx configuration files")
	cmd.Flags().StringVarP(&outputDir, "output", "o", os.Getenv("NIMO_OUTPUT"), "Output directory for spec.json (can also be set via NIMO_OUTPUT env var)")
	cmd.Flags().StringVar(&docsDir, "docs", "", "Output directory for VitePress markdown files (overrides output dir)")
	cmd.Flags().BoolVarP(&generateDocs, "generate-docs", "d", false, "Generate VitePress markdown files in output directory")
	cmd.Flags().StringVarP(&indexPath, "index", "i", "", "Path to generate/update VitePress index.md with features")
	cmd.Flags().StringVar(&filePrefix, "file-prefix", "", "Prefix for generated file names")
	cmd.Flags().StringVar(&commonPrefix, "common-prefix", "", "URL path prefix for VitePress documentation links")
	cmd.Flags().BoolVar(&writeIntroduction, "write-introduction", false, "Generate introduction page for API documentation")
	cmd.Flags().BoolVar(&mergeResponsesInline, "merge-responses-inline", false, "Merge allOf response definitions into single inline objects")
	cmd.Flags().BoolVar(&inlineExamples, "inline-examples", false, "Inline #/components/examples/* refs at every consumption site and drop the components.examples map")
	cmd.Flags().BoolVar(&inlineResponses, "inline-responses", false, "Inline #/components/responses/* refs at every consumption site and drop the components.responses map")
	cmd.Flags().BoolVar(&inlineSchemas, "inline-schemas", false, "Inline #/components/schemas/* refs at every consumption site and drop the components.schemas map. Circular schemas keep their ref")
	cmd.Flags().BoolVar(&flat, "flat", false, "Shortcut for --inline-examples --inline-responses --inline-schemas")

	return cmd
}

type InlineOptions struct {
	Examples  bool
	Responses bool
	Schemas   bool
}

func (o InlineOptions) Any() bool {
	return o.Examples || o.Responses || o.Schemas
}

func RunConvert(args []string, nginxOutput, specOutput, docsPath string, genDocs bool, indexFilePath, filePrefixStr, commonPrefixStr string, writeIntro, mergeResponses bool, inline InlineOptions) error {
	if nginxOutput != "" {
		if err := os.MkdirAll(nginxOutput, 0755); err != nil {
			return fmt.Errorf("failed to create nginx output directory: %w", err)
		}
	}

	for _, path := range args {
		if err := processPath(path, nginxOutput, specOutput, docsPath, genDocs, indexFilePath, filePrefixStr, commonPrefixStr, writeIntro, mergeResponses, inline); err != nil {
			return err
		}
	}

	return nil
}

func runConvertCommand(cmd *cobra.Command, args []string) error {
	inline := InlineOptions{
		Examples:  inlineExamples || flat,
		Responses: inlineResponses || flat,
		Schemas:   inlineSchemas || flat,
	}
	return RunConvert(args, nginxOutputDir, outputDir, docsDir, generateDocs, indexPath, filePrefix, commonPrefix, writeIntroduction, mergeResponsesInline, inline)
}

func processPath(pattern string, nginxOutput, specOutput, docsPath string, genDocs bool, indexFilePath, filePrefixStr, commonPrefixStr string, writeIntro, mergeResponses bool, inline InlineOptions) error {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	
	if len(matches) == 0 {
		return fmt.Errorf("no matches found for pattern: %s", pattern)
	}
	
	for _, path := range matches {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return err
		}
		
		if fileInfo.IsDir() {
			err = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && (strings.HasSuffix(info.Name(), ".yml") || strings.HasSuffix(info.Name(), ".yaml")) {
					return processFile(path, nginxOutput, specOutput, docsPath, genDocs, indexFilePath, filePrefixStr, commonPrefixStr, writeIntro, mergeResponses, inline)
				}
				return nil
			})
			if err != nil {
				return err
			}
		} else if strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml") {
			err = processFile(path, nginxOutput, specOutput, docsPath, genDocs, indexFilePath, filePrefixStr, commonPrefixStr, writeIntro, mergeResponses, inline)
			if err != nil {
				return err
			}
		}
	}
	
	return nil
}

func processFile(filePath string, nginxOutput, specOutput, docsPath string, genDocs bool, indexFilePath, filePrefixStr, commonPrefixStr string, writeIntro, mergeResponses bool, inline InlineOptions) error {
	conv, err := converter.NewOpenApiConverter(filePath)
	if err != nil {
		return fmt.Errorf("failed to load OpenAPI specification: %w", err)
	}

	conv.FilePrefix = filePrefixStr
	conv.WriteIntroduction = writeIntro
	conv.CommonPrefix = commonPrefixStr

	if err = conv.ValidateDocument(); err != nil {
		return fmt.Errorf("validation error: %s", err)
	}

	if mergeResponses {
		err = conv.MergeResponsesInline()
		if err != nil {
			return fmt.Errorf("merge error: %s", err)
		}
		fmt.Printf("✓ Merged response definitions for %s\n", filePath)
	}

	if inline.Examples {
		if err = conv.InlineExamples(); err != nil {
			return fmt.Errorf("inline examples error: %s", err)
		}
		fmt.Printf("✓ Inlined examples for %s\n", filePath)
	}
	if inline.Responses {
		if err = conv.InlineResponses(); err != nil {
			return fmt.Errorf("inline responses error: %s", err)
		}
		fmt.Printf("✓ Inlined responses for %s\n", filePath)
	}
	if inline.Schemas {
		if err = conv.InlineSchemas(); err != nil {
			return fmt.Errorf("inline schemas error: %s", err)
		}
		fmt.Printf("✓ Inlined schemas for %s\n", filePath)
	}

	if nginxOutput != "" {
		config, err := conv.WriteNginxConfiguration()
		if err != nil {
			return fmt.Errorf("failed to generate Nginx config: %w", err)
		}

		outputFile := filepath.Join(nginxOutput, filepath.Base(filePath[:len(filePath)-len(filepath.Ext(filePath))])+".conf.template")
		if err := os.WriteFile(outputFile, []byte(config), 0644); err != nil {
			return fmt.Errorf("failed to write Nginx config: %w", err)
		}

		fmt.Printf("✓ Generated Nginx config: %s\n", outputFile)
	}
	
	outputDirForSpec := filepath.Dir(filePath)
	if len(specOutput) > 0 {
		outputDirForSpec = specOutput
	} else if len(docsPath) > 0 {
		outputDirForSpec = docsPath
	}

	err = conv.WriteOpenAPISpec(outputDirForSpec)
	if err != nil {
		return fmt.Errorf("failed to write OpenAPI spec: %w", err)
	}

	if genDocs || len(docsPath) > 0 {
		docsOutputPath := docsPath
		if len(docsOutputPath) == 0 {
			docsOutputPath = specOutput
		}
		if len(docsOutputPath) == 0 {
			docsOutputPath = filepath.Dir(filePath)
		}

		err = conv.WriteVitePressMarkdown(docsOutputPath)
		if err != nil {
			return fmt.Errorf("failed to write VitePress docs: %w", err)
		}
		fmt.Printf("✓ Generated VitePress docs in %s\n", docsOutputPath)
	}

	if indexFilePath != "" {
		err = conv.WriteVitePressFeatures(indexFilePath)
		if err != nil {
			return fmt.Errorf("failed to write index: %w", err)
		}
		fmt.Printf("✓ Updated index features in %s\n", indexFilePath)
	}

	return nil
}