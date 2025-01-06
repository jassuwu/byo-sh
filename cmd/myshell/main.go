package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	// REPL
	for {
		// Uncomment this block to pass the first stage
		fmt.Fprint(os.Stdout, "$ ")

		// Wait for user input
		commandWithNewLine, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input:", err)
		}
		command := commandWithNewLine[:len(commandWithNewLine)-1]
		commandAndArgs := strings.Split(command, " ")
		if commandAndArgs[0] == "exit" {
			break
		} else if commandAndArgs[0] == "echo" {
			fmt.Println(strings.Join(commandAndArgs[1:], " "))
		} else {
			fmt.Println(command + ": command not found")
		}
	}
}
