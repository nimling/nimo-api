package internal

import (
	"fmt"
	"os"
	"strconv"

	"github.com/nimling/nimo-api/pkg/merger"
	"github.com/spf13/cobra"
)

var (
	mergeOutput      string
	mergeForce       bool
	mergeTitle       string
	mergeDescription string
	mergeVersion     string
	mergeContactName string
	mergeContactEmail string
	mergeFormat      string
	mergeStrategy    string
	mergeServer      string
	mergeTextFormat  string
)

func NewMergeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merge [spec-files...]",
		Short: "Merge multiple OpenAPI specifications",
		Long: `Merge multiple OpenAPI specification files into a single unified spec.

Environment Variables:
  MERGE_OUTPUT      - Default output file path (default: api.spec.json)
  FORCE_OVERWRITE   - Force overwrite existing file (true/false)
  API_TITLE         - Default API title
  API_DESCRIPTION   - Default API description
  API_VERSION       - Default API version
  VERSION           - Fallback for API_VERSION
  CONTACT_NAME      - Default contact name
  CONTACT_EMAIL     - Default contact email
  OUTPUT_FORMAT     - Default output format: yaml or json (default: json)`,
		Example: `  nimo merge spec1.json spec2.json spec3.json -o merged.json
  nimo merge *.json --title "My API" --version "v1.0.0" -o api.json
  export API_TITLE="Company API"
  export CONTACT_NAME="API Team"
  export CONTACT_EMAIL="api@example.com"
  nimo merge spec1.json spec2.json -o api.json`,
		RunE: runMergeCommand,
		Args: cobra.MinimumNArgs(1),
	}

	cmd.Flags().StringVarP(&mergeOutput, "output", "o", getEnvOrDefault("MERGE_OUTPUT", "api.spec.json"), "Output file path")
	cmd.Flags().BoolVarP(&mergeForce, "force", "f", getEnvBool("FORCE_OVERWRITE", false), "Force overwrite existing file")
	cmd.Flags().StringVar(&mergeTitle, "title", getEnvOrDefault("API_TITLE", ""), "Override API title")
	cmd.Flags().StringVar(&mergeDescription, "description", getEnvOrDefault("API_DESCRIPTION", ""), "Override API description")
	cmd.Flags().StringVar(&mergeVersion, "api-version", getVersionFromEnv(), "Override API version")
	cmd.Flags().StringVar(&mergeContactName, "contact-name", getEnvOrDefault("CONTACT_NAME", ""), "Override contact name")
	cmd.Flags().StringVar(&mergeContactEmail, "contact-email", getEnvOrDefault("CONTACT_EMAIL", ""), "Override contact email")
	cmd.Flags().StringVar(&mergeFormat, "format", getEnvOrDefault("OUTPUT_FORMAT", "json"), "Output format (yaml or json)")
	cmd.Flags().StringVar(&mergeStrategy, "strategy", getEnvOrDefault("MERGE_STRATEGY", "last"), "Conflict resolution strategy: 'first' or 'last'")
	cmd.Flags().StringVar(&mergeServer, "server", getEnvOrDefault("API_SERVER", ""), "Override server URL")
	cmd.Flags().StringVar(&mergeTextFormat, "text-format", getEnvOrDefault("TEXT_FORMAT", "asis"), "Text format for all descriptions and summaries: 'asis', 'html', or 'markdown'")

	return cmd
}

func runMergeCommand(cmd *cobra.Command, args []string) error {
	fmt.Printf("Merging %d OpenAPI specs...\n", len(args))

	if mergeStrategy != "first" && mergeStrategy != "last" {
		return fmt.Errorf("invalid strategy: %s (must be 'first' or 'last')", mergeStrategy)
	}

	if mergeTextFormat != "asis" && mergeTextFormat != "html" && mergeTextFormat != "markdown" {
		return fmt.Errorf("invalid text-format: %s (must be 'asis', 'html', or 'markdown')", mergeTextFormat)
	}

	opts := merger.MergeOptions{
		Title:        mergeTitle,
		Description:  mergeDescription,
		Version:      mergeVersion,
		ContactName:  mergeContactName,
		ContactEmail: mergeContactEmail,
		Force:        mergeForce,
		OutputFormat: mergeFormat,
		Strategy:     mergeStrategy,
		ServerURL:    mergeServer,
		TextFormat:   mergeTextFormat,
	}

	return merger.MergeSpecs(args, mergeOutput, opts)
}

func getVersionFromEnv() string {
	if v := os.Getenv("API_VERSION"); v != "" {
		return v
	}
	if v := os.Getenv("VERSION"); v != "" {
		return v
	}
	return ""
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
