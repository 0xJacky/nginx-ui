package version

var (
	Version    = ""
	BuildId    = 0
	TotalBuild = 0
	Hash       = ""
)

type Info struct {
	Version    string `json:"version"`
	BuildId    int    `json:"build_id"`
	TotalBuild int    `json:"total_build"`
}

var versionInfo *Info

func GetVersionInfo() *Info {
	if versionInfo == nil {
		versionInfo = &Info{
			Version:    Version,
			BuildId:    BuildId,
			TotalBuild: TotalBuild,
		}
	}
	return versionInfo
}

func GetShortHash() string {
	if Hash != "" {
		return Hash[:8]
	}
	return ""
}
