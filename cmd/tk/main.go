// cmd/threadkeeper/main.go
package main

import (
	"os"

	"github.com/sjatkinson/threadkeeper/internal/cli"
)

func main() {
	code := cli.Run(os.Args[1:], cli.Config{
		AppName: "tk",
		Version: "0.1.0-dev",
	})
	os.Exit(code)
}
