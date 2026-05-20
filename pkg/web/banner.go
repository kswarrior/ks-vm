package web

import "fmt"

const Banner = `
 ██╗  ██╗███████╗██╗   ██╗███╗   ███╗
 ██║ ██╔╝██╔════╝██║   ██║████╗ ████║
 █████╔╝ ███████╗██║   ██║██╔████╔██║
 ██╔═██╗ ╚════██║╚██╗ ██╔╝██║╚██╔╝██║
 ██║  ██╗███████║ ╚████╔╝ ██║ ╚═╝ ██║
 ╚═╝  ╚═╝╚══════╝  ╚═══╝  ╚═╝     ╚═╝
`

func PrintShellBanner(name string) {
	fmt.Printf("\x1b[36m%s\x1b[0m\n", Banner)
	fmt.Printf("\x1b[1;35m--- SESSION ESTABLISHED: %s ---\x1b[0m\n", name)
	fmt.Printf("\x1b[33mType 'exit' or press Ctrl+C to disconnect\x1b[0m\n\n")
}
