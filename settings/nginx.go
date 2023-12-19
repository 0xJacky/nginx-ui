package settings

type Nginx struct {
	AccessLogPath string `json:"access_log_path"`
	ErrorLogPath  string `json:"error_log_path"`
	ConfigDir     string `json:"config_dir" protected:"true"`
	PIDPath       string `json:"pid_path" protected:"true"`
	TestConfigCmd string `json:"test_config_cmd" protected:"true"`
	ReloadCmd     string `json:"reload_cmd" protected:"true"`
	RestartCmd    string `json:"restart_cmd" protected:"true"`
}

var NginxSettings = Nginx{
	AccessLogPath: "",
	ErrorLogPath:  "",
}
