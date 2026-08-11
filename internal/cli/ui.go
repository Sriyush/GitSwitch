package cli

import (
	"flag"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/sriyush/gitswitch/internal/server"
)

// cmdUI starts the local management server and opens the browser at it.
//
// The URL carries a session token minted at startup; without it the API refuses
// every request. That is why the URL is printed as well as opened — if the
// browser launch fails, or opens in the wrong profile, the user needs the whole
// URL rather than just the port.
func cmdUI(args []string) error {
	fs := flag.NewFlagSet("ui", flag.ContinueOnError)
	port := fs.Int("port", 0, "port to bind on 127.0.0.1 (default 7842, or any free port)")
	noOpen := fs.Bool("no-open", false, "print the URL instead of opening a browser")
	if err := fs.Parse(args); err != nil {
		return err
	}

	srv, err := server.New(*port)
	if err != nil {
		return err
	}

	url := srv.URL()
	fmt.Printf("gitswitch UI on http://127.0.0.1:%d\n", srv.Port())
	fmt.Printf("  %s\n\n", url)
	fmt.Println("Bound to 127.0.0.1 only. The session token in that URL lives in memory")
	fmt.Println("and dies with this process. Press Ctrl-C to stop.")

	if !*noOpen {
		if err := openBrowser(url); err != nil {
			fmt.Printf("\nCould not open a browser (%v). Open the URL above yourself.\n", err)
		}
	}
	return srv.Serve()
}

// openBrowser launches the platform's URL handler.
//
// The process is started and released rather than waited on: xdg-open exits
// immediately on some desktops and blocks for the life of the browser on
// others, and `gsw ui` must not depend on which.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
