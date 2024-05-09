package main

import (
	"github.com/curioswitch/go-build"
	"github.com/goyek/x/boot"
)

func main() {
	build.DefineTasks(
		build.LocalPackagePrefix("github.com/curioswitch/go-usegcp"),
	)
	boot.Main()
}
