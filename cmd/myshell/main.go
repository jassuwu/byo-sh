package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	builtins := []string{"exit", "echo", "type"}
	// REPL
REPL:
	for {
		// Uncomment this block to pass the first stage
		fmt.Fprint(os.Stdout, "$ ")

		// Wait for user input
		commandWithNewLine, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input:", err)
		}
		commandString := commandWithNewLine[:len(commandWithNewLine)-1]
		commandAndArgs := strings.Split(commandString, " ")
		switch commandAndArgs[0] {
		case "exit":
			break REPL
		case "echo":
			fmt.Println(strings.Join(commandAndArgs[1:], " "))
		case "type":
			commandToFindType, found := commandAndArgs[1], false
			for _, builtin := range builtins {
				if builtin == commandToFindType {
					fmt.Println(commandString, "is a shell builtin")
					found = true
					break
				}
			}
			if !found {
				fmt.Println(commandString + ": not found")
			}
		default:
			fmt.Println(commandString + ": command not found")
		}
	}
}
