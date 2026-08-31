package main

import (
	"fmt"
	"os"

	"github.com/nimling/nimo-api/internal"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "nimo",
		Short:   "OpenAPI toolkit",
		Version: internal.GetVersion(),
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true

	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		internal.PrintBanner()
		defaultHelp(cmd, args)
	})

	rootCmd.AddCommand(internal.NewGenerateCommand())
	rootCmd.AddCommand(internal.NewConvertCommand())
	rootCmd.AddCommand(internal.NewMergeCommand())
	rootCmd.AddCommand(internal.NewSyncCommand())
	rootCmd.AddCommand(internal.NewSkillCommand())
	rootCmd.AddCommand(internal.NewCompletionCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
