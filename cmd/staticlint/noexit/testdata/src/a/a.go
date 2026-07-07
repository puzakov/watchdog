package main

import "os"

func main() {
	os.Exit(0) // want "direct call to os.Exit is forbidden in main function of package main"
}

func helper() {
	os.Exit(0) // OK — not in main function
}
