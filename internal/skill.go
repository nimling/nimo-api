package internal

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed skill/SKILL.md
var SkillDocument string

var skillProject bool

func NewSkillCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "The claude skill that teaches an agent to drive this cli",
	}

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Print the skill document",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(cmd.OutOrStdout(), SkillDocument)
			return nil
		},
	}

	putCmd := &cobra.Command{
		Use:   "put",
		Short: "Install the skill under ~/.claude/skills, or into the current project with --project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return installSkill(cmd, !skillProject)
		},
	}

	putCmd.Flags().BoolVar(&skillProject, "project", false, "install into .claude of the current directory instead of the home directory")
	cmd.AddCommand(getCmd, putCmd)

	return cmd
}

func installSkill(cmd *cobra.Command, global bool) error {
	root := ".claude"
	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		root = filepath.Join(home, ".claude")
	}

	dir := filepath.Join(root, "skills", "nimo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(SkillDocument), 0o644); err != nil {
		return err
	}

	cmd.Println("wrote " + path)
	return nil
}
