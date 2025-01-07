package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
)

func main() {
	builtins := []string{"exit", "echo", "type"}
	PATH := os.Getenv("PATH")
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
					fmt.Println(commandToFindType, "is a shell builtin")
					found = true
					break
				}
			}
			if !found {
				paths := strings.Split(PATH, ":")
				for _, path := range paths {
					dirEntries, _ := os.ReadDir(path)
					// if err != nil {
					// 	fmt.Fprintln(os.Stderr, "Error reading directory entries:", err)
					// }
					for _, commandInPath := range dirEntries {
						if !commandInPath.IsDir() && commandToFindType == commandInPath.Name() {
							fmt.Println(commandToFindType, "is", path+"/"+commandToFindType)
							found = true
							break
						}
					}
				}
			}
			if !found {
				fmt.Println(commandToFindType + ": not found")
			}
		default:
			found := false
			paths := strings.Split(PATH, ":")
		PATHLOOP:
			for _, path := range paths {
				dirEntries, _ := os.ReadDir(path)
				for _, commandInPath := range dirEntries {
					if !commandInPath.IsDir() && commandInPath.Name() == commandAndArgs[0] {
						execErr := syscall.Exec(path+"/"+commandAndArgs[0], commandAndArgs, os.Environ())
						if execErr != nil {
							fmt.Fprintln(os.Stderr, execErr)
						}
						found = true
						break PATHLOOP
					}
				}
			}
			if !found {
				fmt.Println(commandString + ": command not found")
			}
		}
	}
}
