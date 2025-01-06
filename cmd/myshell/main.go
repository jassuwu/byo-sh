package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	// REPL
	for {
		// Uncomment this block to pass the first stage
		fmt.Fprint(os.Stdout, "$ ")

		// Wait for user input
		command, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input:", err)
		}
		fmt.Println(command[:len(command)-1] + ": command not found")
	}
}
