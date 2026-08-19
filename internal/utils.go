package internal

import (
	"fmt"
)

var Version = "dev"

func GetVersion() string {
	return Version
}

func PrintBanner() {
	banner := fmt.Sprintf(`
███╗   ██╗██╗███╗   ███╗ ██████╗
████╗  ██║██║████╗ ████║██╔═══██╗
██╔██╗ ██║██║██╔████╔██║██║   ██║
██║╚██╗██║██║██║╚██╔╝██║██║   ██║
██║ ╚████║██║██║ ╚═╝ ██║╚██████╔╝
╚═╝  ╚═══╝╚═╝╚═╝     ╚═╝ ╚═════╝
       %s
`, Version)
	fmt.Println(banner)
}
