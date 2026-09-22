package settings

import "runtime"

type Terminal struct {
	StartCmd string `json:"start_cmd" protected:"true"`
}

var TerminalSettings = &Terminal{
	StartCmd: defaultTerminalStartCmd(runtime.GOOS),
}

func defaultTerminalStartCmd(goos string) string {
	if goos == "windows" {
		return "cmd.exe"
	}
	return "login"
}
