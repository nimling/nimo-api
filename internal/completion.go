package internal

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	completionAuto  bool
	completionSkill bool
)

func NewCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Print the shell completion script, or wire it up with --auto",
		Long: `Print the completion script for one shell to stdout, or pass --auto to install it.

--auto detects the shell from $SHELL when no shell is given, writes a nimo owned
completion file under ~/.config/nimo/completions, and points the shell rc file at
it inside a managed block. Rerunning --auto refreshes the nimo owned file and
leaves the rc untouched. fish loads from its own completions directory and needs
no rc change.

--skill installs the claude skill in the same step, globally under ~/.claude/skills.

  nimo completion zsh             print the zsh script
  nimo completion --auto          detect the shell and install
  nimo completion --auto --skill  install completion and the claude skill
  nimo completion bash --auto     install for a named shell`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := ""
			if len(args) == 1 {
				shell = args[0]
			}

			if !completionAuto {
				if completionSkill {
					return errors.New("--skill installs alongside --auto")
				}
				if shell == "" {
					return errors.New("name a shell (bash, zsh, fish, powershell) or pass --auto")
				}
				return writeCompletionScript(cmd.Root(), shell, cmd.OutOrStdout())
			}

			if err := installCompletion(cmd, shell); err != nil {
				return err
			}
			if completionSkill {
				return installSkill(cmd, true)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&completionAuto, "auto", false, "detect the shell and install completion into the rc file")
	cmd.Flags().BoolVar(&completionSkill, "skill", false, "install the claude skill globally in the same step")

	return cmd
}

func writeCompletionScript(root *cobra.Command, shell string, out io.Writer) error {
	switch shell {
	case "bash":
		return root.GenBashCompletionV2(out, true)
	case "zsh":
		return root.GenZshCompletion(out)
	case "fish":
		return root.GenFishCompletion(out, true)
	case "powershell":
		return root.GenPowerShellCompletionWithDesc(out)
	default:
		return errors.New("unsupported shell " + shell + ", use bash, zsh, fish, or powershell")
	}
}

func detectShell() string {
	base := filepath.Base(os.Getenv("SHELL"))
	switch {
	case strings.Contains(base, "zsh"):
		return "zsh"
	case strings.Contains(base, "bash"):
		return "bash"
	case strings.Contains(base, "fish"):
		return "fish"
	}
	return ""
}

func installCompletion(cmd *cobra.Command, shell string) error {
	if shell == "" {
		shell = detectShell()
	}
	if shell == "" {
		return errors.New("could not detect the shell from $SHELL, name it: nimo completion <shell> --auto")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	var script bytes.Buffer
	if err := writeCompletionScript(cmd.Root(), shell, &script); err != nil {
		return err
	}

	if shell == "fish" {
		dir := filepath.Join(home, ".config", "fish", "completions")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		path := filepath.Join(dir, "nimo.fish")
		if err := os.WriteFile(path, script.Bytes(), 0o644); err != nil {
			return err
		}
		cmd.Println("wrote " + path)
		cmd.Println("fish loads it on the next shell")
		return nil
	}

	if shell != "bash" && shell != "zsh" {
		return errors.New("auto install covers bash, zsh, and fish, for " + shell + " run: nimo completion " + shell)
	}

	dir := filepath.Join(home, ".config", "nimo", "completions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	name := "nimo.bash"
	if shell == "zsh" {
		name = "_nimo"
	}
	scriptPath := filepath.Join(dir, name)
	if err := os.WriteFile(scriptPath, script.Bytes(), 0o644); err != nil {
		return err
	}
	cmd.Println("wrote " + scriptPath)

	rc := filepath.Join(home, ".bashrc")
	body := fmt.Sprintf("[ -f %q ] && source %q", scriptPath, scriptPath)
	if shell == "zsh" {
		rc = filepath.Join(home, ".zshrc")
		body = fmt.Sprintf("fpath=(%q $fpath)\nautoload -Uz compinit && compinit -u", dir)
	}
	if err := ensureManagedBlock(rc, body); err != nil {
		return err
	}

	cmd.Println("wired " + rc)
	cmd.Println("start a new shell or run: source " + rc)
	return nil
}

func ensureManagedBlock(path string, body string) error {
	const start = "# >>> nimo completion >>>"
	const end = "# <<< nimo completion <<<"
	block := start + "\n" + body + "\n" + end

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(data)

	if opening := strings.Index(content, start); opening >= 0 {
		if closing := strings.Index(content, end); closing > opening {
			content = content[:opening] + block + content[closing+len(end):]
		} else {
			content = strings.TrimRight(content, "\n") + "\n" + block + "\n"
		}
	} else {
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += block + "\n"
	}

	return os.WriteFile(path, []byte(content), 0o644)
}
