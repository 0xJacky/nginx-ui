package settings

type Nginx struct {
	AccessLogPath            string   `json:"access_log_path" protected:"true"`
	ErrorLogPath             string   `json:"error_log_path" protected:"true"`
	LogDirWhiteList          []string `json:"log_dir_white_list" protected:"true"`
	ConfigDir                string   `json:"config_dir" protected:"true"`
	ConfigPath               string   `json:"config_path" protected:"true"`
	PIDPath                  string   `json:"pid_path" protected:"true"`
	SbinPath                 string   `json:"sbin_path" protected:"true"`
	TestConfigCmd            string   `json:"test_config_cmd" protected:"true"`
	ReloadCmd                string   `json:"reload_cmd" protected:"true"`
	RestartCmd               string   `json:"restart_cmd" protected:"true"`
	StubStatusPort           uint     `json:"stub_status_port" binding:"omitempty,min=1,max=65535"`
	ContainerName            string   `json:"container_name" protected:"true"`
}

var NginxSettings = &Nginx{}

func (n *Nginx) GetStubStatusPort() uint {
	if n.StubStatusPort == 0 {
		return 51820
	}
	return n.StubStatusPort
}

func (n *Nginx) RunningInAnotherContainer() bool {
	return n.ContainerName != ""
}
