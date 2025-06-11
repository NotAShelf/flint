package main

import (
	"notashelf.dev/flint/cmd"
)

var version string

func main() {
	cmd.Version = version
	cmd.Execute()
}
