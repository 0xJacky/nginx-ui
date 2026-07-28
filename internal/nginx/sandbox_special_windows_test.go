//go:build windows

package nginx

func createSandboxSpecialEntry(string) (string, error) {
	return "", nil
}
