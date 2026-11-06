// claude-bk backs up and restores the Claude Code conversations of the
// current working directory to a provider store.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const usage = `claude-bk <provider> <push|pull>

Back up this directory's Claude Code conversations.

<provider> selects ~/.claude-bk/<provider>.json, which describes where the
archive is stored. The project is derived from the current working directory,
so run claude-bk from the same directory you start Claude Code in.

  push    pack this project's conversations and upload them, replacing any
          previous archive of the same project
  pull    download this project's archive and unpack it in place

Examples:
  cd /home/xin/mg4
  claude-bk nas push
  claude-bk nas pull
`

func main() {
	root := &cobra.Command{
		Use:           "claude-bk <provider> <push|pull>",
		Short:         "Back up Claude Code conversations for the current directory",
		Long:          usage,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          run,
	}
	root.SetHelpTemplate(usage)
	root.SetUsageTemplate(usage)
	root.SetHelpFunc(func(*cobra.Command, []string) { fmt.Print(usage) })

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return fmt.Errorf("provider is required")
	}
	if len(args) < 2 {
		fmt.Print(usage)
		return fmt.Errorf("action is required (push or pull)")
	}
	if len(args) > 2 {
		return fmt.Errorf("unexpected argument %q", args[2])
	}

	providerName, action := args[0], args[1]
	switch action {
	case "push":
		return runPush(providerName)
	case "pull":
		return runPull(providerName)
	default:
		return fmt.Errorf("unknown action %q (expected push or pull)", action)
	}
}
