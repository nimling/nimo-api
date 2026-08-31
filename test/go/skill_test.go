package test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nimling/nimo-api/internal"
	"github.com/spf13/cobra"
)

func TestSkillGetPrintsDocument(t *testing.T) {
	cmd := internal.NewSkillCommand()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"get"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill get failed: %v", err)
	}

	if out.String() != internal.SkillDocument {
		t.Fatalf("skill get did not print the embedded document")
	}

	if !strings.Contains(internal.SkillDocument, "name: nimo") {
		t.Fatalf("skill document is missing its frontmatter name")
	}
}

func TestSkillPutProjectWritesFile(t *testing.T) {
	dir := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	defer os.Chdir(previous)

	cmd := internal.NewSkillCommand()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"put", "--project"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("skill put --project failed: %v", err)
	}

	path := filepath.Join(".claude", "skills", "nimo", "SKILL.md")
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("skill put did not write %s: %v", path, err)
	}

	if string(written) != internal.SkillDocument {
		t.Fatalf("written skill does not match the embedded document")
	}

	if !strings.Contains(out.String(), "wrote "+path) {
		t.Fatalf("skill put did not report the path, got %q", out.String())
	}
}

func TestCompletionScriptPerShell(t *testing.T) {
	shells := map[string]string{
		"bash":       "bash completion V2",
		"zsh":        "compdef",
		"fish":       "nimo",
		"powershell": "Register-ArgumentCompleter",
	}

	for shell, marker := range shells {
		root := &cobra.Command{Use: "nimo"}
		root.CompletionOptions.DisableDefaultCmd = true
		completion := internal.NewCompletionCommand()
		root.AddCommand(completion)

		out := &bytes.Buffer{}
		root.SetOut(out)
		root.SetErr(out)
		root.SetArgs([]string{"completion", shell})

		if err := root.Execute(); err != nil {
			t.Fatalf("completion %s failed: %v", shell, err)
		}

		if !strings.Contains(out.String(), marker) {
			t.Fatalf("completion %s did not print a %s script", shell, shell)
		}
	}
}

func TestCompletionRejectsMissingShell(t *testing.T) {
	root := &cobra.Command{Use: "nimo"}
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(internal.NewCompletionCommand())

	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"completion"})

	err := root.Execute()
	if err == nil {
		t.Fatalf("completion with no shell and no --auto should fail")
	}
	if !strings.Contains(err.Error(), "name a shell") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompletionRejectsUnknownShell(t *testing.T) {
	root := &cobra.Command{Use: "nimo"}
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(internal.NewCompletionCommand())

	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"completion", "tcsh"})

	err := root.Execute()
	if err == nil {
		t.Fatalf("completion with an unknown shell should fail")
	}
	if !strings.Contains(err.Error(), "unsupported shell tcsh") {
		t.Fatalf("unexpected error: %v", err)
	}
}
