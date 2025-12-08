package internal

import (
	"fmt"
	"os"
	"strings"
)

var Version = "dev"

func GetVersion() string {
	if Version != "dev" {
		return Version
	}

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