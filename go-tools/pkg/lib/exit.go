package lib

import (
	"fmt"
	"os"
)

// Exit prints the error and exits the program with code 1
func Exit(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}
