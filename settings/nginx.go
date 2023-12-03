package settings

type Nginx struct {
	AccessLogPath string `json:"access_log_path"`
	ErrorLogPath  string `json:"error_log_path"`
	ConfigDir     string `json:"config_dir"`
	PIDPath       string `json:"pid_path"`
	TestConfigCmd string `json:"test_config_cmd"`
	ReloadCmd     string `json:"reload_cmd"`
	RestartCmd    string `json:"restart_cmd"`
}

var NginxSettings = Nginx{
	AccessLogPath: "",
	ErrorLogPath:  "",
}
