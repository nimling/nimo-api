package internal

import (
	"fmt"
	"os"
	"strings"
)

var Version = "v2.0.0"

func GetVersion() string {
	data, err := os.ReadFile(".env")
	if err != nil {
		return Version
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "APP_VERSION=") {
			return strings.TrimPrefix(line, "APP_VERSION=")
		}
	}

	return Version
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
       %s
`, version)
	fmt.Println(banner)
}