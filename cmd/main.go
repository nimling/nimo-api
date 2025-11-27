package main

import (
	"fmt"
	"os"

	"github.com/nimling/nimo-api/internal"
	"github.com/spf13/cobra"
)

func main() {
	internal.PrintBanner()

	rootCmd := &cobra.Command{
		Use:   "nimo",
		Short: "Nimo - Complete OpenAPI toolkit for generation, conversion, merging, and synchronization",
		Long: `Nimo is a comprehensive CLI tool for working with OpenAPI specifications.

It provides four main capabilities:
- Generate: Create OpenAPI specs from Go Echo handler code using AI
- Convert: Transform OpenAPI specs into Nginx configurations and VitePress documentation
- Merge: Combine multiple OpenAPI specifications into one unified spec
- Sync: Synchronize documentation files across projects using pattern-based mapping

Perfect for maintaining consistent API documentation across microservices and documentation workflows.`,
		Version: internal.GetVersion(),
	}

	rootCmd.AddCommand(internal.NewGenerateCommand())
	rootCmd.AddCommand(internal.NewConvertCommand())
	rootCmd.AddCommand(internal.NewMergeCommand())
	rootCmd.AddCommand(internal.NewSyncCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
