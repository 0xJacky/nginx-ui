package settings

type Terminal struct {
	StartCmd string `json:"start_cmd" protected:"true"`
}

var TerminalSettings = &Terminal{}
