package internal

import (
	"fmt"

	"github.com/nimling/nimo-api/converter"
	"github.com/spf13/cobra"
)

var syncMapFile string

func NewSyncCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize documentation files using pattern mapping",
		Long: `Synchronize documentation files between projects using regexp pattern matching.

The sync command uses a JSON mapping file to copy documentation from source
locations to target destinations. It supports:
- Regular expression patterns for flexible file matching
- Automatic index.md detection in directories
- Multiple pattern fallbacks per destination
- Batch processing with detailed error reporting

Mapping File Format:
  {
    "output/guides/project/": [
      ".*docs/guide\\.md$",
      ".*docs/guide/index\\.md$"
    ],
    "output/api/reference/index.md": [
      ".*api/reference\\.md$"
    ]
  }

Examples:
  # Sync documentation using mapping file
  nimo sync -s mapping.json

  # Sync with custom mapping
  nimo sync --sync-map ./config/sync-map.json`,
		RunE: runSyncCommand,
	}
	
	cmd.Flags().StringVarP(&syncMapFile, "sync-map", "s", "", "JSON file containing source-to-destination mapping rules (required)")
	cmd.MarkFlagRequired("sync-map")
	
	return cmd
}

func RunSync(mappingFilePath string) error {
	syncer, err := converter.NewDocSyncer(mappingFilePath)
	if err != nil {
		return fmt.Errorf("failed to initialize syncer: %w", err)
	}
	
	fmt.Printf("Syncing documentation using mapping: %s\n", mappingFilePath)
	return syncer.Execute()
}

func runSyncCommand(cmd *cobra.Command, args []string) error {
	return RunSync(syncMapFile)
}