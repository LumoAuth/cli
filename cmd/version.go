package cmd

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	// Version is the CLI version, set via -ldflags at build time.
	// Example: go build -ldflags "-X github.com/lumoauth/cli/cmd.Version=1.2.3"
	Version = "dev"

	// Commit is the git commit hash, set via -ldflags at build time.
	Commit = "unknown"

	// Date is the build date, set via -ldflags at build time.
	Date = "unknown"
)

func init() {
	rootCmd.AddCommand(versionCmd)

	rootCmd.Version = versionString()
	rootCmd.SetVersionTemplate(`{{.Version}}` + "\n")
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the CLI version, commit, and build info",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("lumo %s\n", Version)
		fmt.Printf("  commit: %s\n", commit())
		fmt.Printf("  built:  %s\n", Date)
		fmt.Printf("  go:     %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	},
}

// versionString is what `lumo --version` prints. Concise single-line form.
func versionString() string {
	return fmt.Sprintf("%s (commit %s, built %s, %s)", Version, shortCommit(commit()), Date, runtime.Version())
}

// commit resolves to the ldflags-injected value, or falls back to debug.BuildInfo
// (which captures the VCS revision when the binary was built with module-mode go).
func commit() string {
	if Commit != "" && Commit != "unknown" {
		return Commit
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				return s.Value
			}
		}
	}
	return "unknown"
}

func shortCommit(c string) string {
	if len(c) >= 7 {
		return c[:7]
	}
	return c
}
