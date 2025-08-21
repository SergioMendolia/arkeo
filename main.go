package main

import (
	"github.com/arkeo/arkeo/cmd"
)

var version = "dev" // Will be set by build process

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
