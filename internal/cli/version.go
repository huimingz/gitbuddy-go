package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print detailed version information about GitBuddy.`,
	Run: func(cmd *cobra.Command, args []string) {
		v, commit, buildTime := GetVersionInfo()
		fmt.Printf("GitBuddy %s\n", v)
		fmt.Printf("  Git Commit: %s\n", commit)
		fmt.Printf("  Build Time: %s\n", buildTime)
		fmt.Printf("  Go Version: %s\n", runtime.Version())
		fmt.Printf("  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
