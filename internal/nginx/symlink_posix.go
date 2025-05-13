//go:build !windows

package nginx

func GetConfSymlinkPath(path string) string {
	return path
}

func GetConfNameBySymlinkName(name string) string {
	return name
}
