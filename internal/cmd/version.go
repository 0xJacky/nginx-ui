package cmd

import (
	"fmt"
	"runtime"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/urfave/cli/v3"
)

func VersionPrinter(cmd *cli.Command) {
	fmt.Println(helper.Concat(
		cmd.Root().Name, " ", cmd.Root().Version, " ", version.BuildId, "(", version.TotalBuild, ") ",
		version.GetShortHash(), " (", runtime.Version(), " ", runtime.GOOS, "/", runtime.GOARCH, ")"))
	fmt.Println(cmd.Root().Usage)
}
