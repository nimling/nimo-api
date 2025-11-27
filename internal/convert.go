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
	outputDir            string
	docsDir              string
	indexPath            string
	filePrefix           string
	commonPrefix         string
	writeIntroduction    bool
	mergeResponsesInline bool
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
	
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory for Nginx configuration files")
	cmd.Flags().StringVarP(&docsDir, "docs", "d", "", "Output directory for VitePress API documentation")
	cmd.Flags().StringVarP(&indexPath, "index", "i", "", "Path to generate/update VitePress index.md with features")
	cmd.Flags().StringVar(&filePrefix, "file-prefix", "", "Prefix for generated file names")
	cmd.Flags().StringVar(&commonPrefix, "common-prefix", "", "URL path prefix for VitePress documentation links")
	cmd.Flags().BoolVar(&writeIntroduction, "write-introduction", false, "Generate introduction page for API documentation")
	cmd.Flags().BoolVar(&mergeResponsesInline, "merge-responses-inline", false, "Merge allOf response definitions into single inline objects")
	
	return cmd
}

func RunConvert(args []string, outputPath, docsPath, indexFilePath, filePrefixStr, commonPrefixStr string, writeIntro, mergeResponses bool) error {
	if outputPath != "" {
		if err := os.MkdirAll(outputPath, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}
	
	for _, path := range args {
		if err := processPath(path, outputPath, docsPath, indexFilePath, filePrefixStr, commonPrefixStr, writeIntro, mergeResponses); err != nil {
			return err
		}
	}
	
	return nil
}

func runConvertCommand(cmd *cobra.Command, args []string) error {
	return RunConvert(args, outputDir, docsDir, indexPath, filePrefix, commonPrefix, writeIntroduction, mergeResponsesInline)
}

func processPath(pattern string, outputPath, docsPath, indexFilePath, filePrefixStr, commonPrefixStr string, writeIntro, mergeResponses bool) error {
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
					return processFile(path, outputPath, docsPath, indexFilePath, filePrefixStr, commonPrefixStr, writeIntro, mergeResponses)
				}
				return nil
			})
			if err != nil {
				return err
			}
		} else if strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml") {
			err = processFile(path, outputPath, docsPath, indexFilePath, filePrefixStr, commonPrefixStr, writeIntro, mergeResponses)
			if err != nil {
				return err
			}
		}
	}
	
	return nil
}

func processFile(filePath string, outputPath, docsPath, indexFilePath, filePrefixStr, commonPrefixStr string, writeIntro, mergeResponses bool) error {
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
	
	if outputPath != "" {
		config, err := conv.WriteNginxConfiguration()
		if err != nil {
			return fmt.Errorf("failed to generate Nginx config: %w", err)
		}
		
		outputFile := filepath.Join(outputPath, filepath.Base(filePath[:len(filePath)-len(filepath.Ext(filePath))])+".conf.template")
		if err := os.WriteFile(outputFile, []byte(config), 0644); err != nil {
			return fmt.Errorf("failed to write Nginx config: %w", err)
		}
		
		fmt.Printf("✓ Generated Nginx config: %s\n", outputFile)
	}
	
	if len(docsPath) > 0 {
		err = conv.WriteVitePressDocs(docsPath)
		if err != nil {
			return fmt.Errorf("failed to write VitePress docs: %w", err)
		}
		fmt.Printf("✓ Generated VitePress docs in %s\n", docsPath)
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