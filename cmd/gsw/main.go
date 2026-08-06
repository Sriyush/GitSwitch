// Command gsw switches the active GitHub identity across every layer git
// authenticates with: commit identity, SSH key, HTTPS token, and signing key.
package main

import (
	"os"

	"github.com/sriyush/gitswitch/internal/cli"
)

// version is overridden at build time via -ldflags.
var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], version))
}
