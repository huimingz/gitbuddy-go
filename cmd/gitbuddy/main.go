/*
Copyright © 2024 huimingz

GitBuddy - AI-powered Git assistant for developers
*/
package main

import (
	"os"

	"github.com/huimingz/gitbuddy-go/internal/cli"
)

// Version information (injected at build time)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	cli.SetVersionInfo(Version, GitCommit, BuildTime)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}

