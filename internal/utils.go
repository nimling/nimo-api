package internal

import (
	"fmt"
	"os"
	"strings"
)

func GetVersion() string {
	data, err := os.ReadFile(".env")
	if err != nil {
		return "v2.0.0"
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "APP_VERSION=") {
			return strings.TrimPrefix(line, "APP_VERSION=")
		}
	}

	return "v2.0.0"
}

func PrintBanner() {
	version := GetVersion()
	banner := fmt.Sprintf(`
███╗   ██╗██╗███╗   ██╗ ██████╗
████╗  ██║██║████╗  ██║██╔═══██╗
██╔██╗ ██║██║██╔██╗ ██║██║   ██║
██║╚██╗██║██║██║╚██╗██║██║   ██║
██║ ╚████║██║██║ ╚████║╚██████╔╝
╚═╝  ╚═══╝╚═╝╚═╝  ╚═══╝ ╚═════╝
Nimo %s
`, version)
	fmt.Println(banner)
}