package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	builtins := []string{"exit", "echo", "type", "pwd", "cd"}
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
		case "pwd":
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			fmt.Println(cwd)
		case "cd":
			newWD := commandAndArgs[1]
			err := os.Chdir(newWD)
			if err != nil {
				fmt.Fprintln(os.Stderr, "cd:", newWD+":", "No such file or directory")
			}
		default:
			found := false
			paths := strings.Split(PATH, ":")
		PATHLOOP:
			for _, path := range paths {
				dirEntries, _ := os.ReadDir(path)
				for _, commandInPath := range dirEntries {
					if !commandInPath.IsDir() && commandInPath.Name() == commandAndArgs[0] {
						commandToExec := exec.Command(path+"/"+commandAndArgs[0], commandAndArgs[1:]...)
						commandToExec.Stdout, commandToExec.Stdin, commandToExec.Stderr = os.Stdout, os.Stdin, os.Stderr
						execErr := commandToExec.Run()
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
