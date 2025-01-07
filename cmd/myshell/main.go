package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func tokenize(s string) []string {
	var tokens []string
	var currentToken strings.Builder
	inQuotes := false

	for i := 0; i < len(s); i++ {
		c := s[i]

		if c == '\'' {
			// Toggle inQuotes state
			inQuotes = !inQuotes
			if !inQuotes {
				// End of quoted segment; continue accumulating for adjacent quotes
				continue
			}
			// Start of quoted segment
			continue
		}

		if inQuotes {
			// Append characters inside quotes to currentToken
			currentToken.WriteByte(c)
		} else {
			if c == ' ' {
				// If outside quotes and space is found, flush currentToken to tokens
				if currentToken.Len() > 0 {
					tokens = append(tokens, currentToken.String())
					currentToken.Reset()
				}
			} else {
				// Append characters outside quotes to currentToken
				currentToken.WriteByte(c)
			}
		}
	}

	// Add the final token if any
	if currentToken.Len() > 0 {
		tokens = append(tokens, currentToken.String())
	}

	return tokens
}

func main() {
	builtins := []string{"exit", "echo", "type", "pwd", "cd"}
	PATH := os.Getenv("PATH")
	// REPL
REPL:
	for {
		fmt.Fprint(os.Stdout, "$ ")
		commandWithNewLine, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input:", err)
		}
		s := strings.Trim(commandWithNewLine, "\r\n")
		tokens := tokenize(s)
		switch tokens[0] {
		case "exit":
			break REPL
		case "echo":
			fmt.Println(strings.Join(tokens[1:], " "))
		case "type":
			commandToFindType, found := tokens[1], false
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
			newWD := tokens[1]
			if newWD == "~" {
				newWD = os.Getenv("HOME")
			}
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
					if !commandInPath.IsDir() && commandInPath.Name() == tokens[0] {
						commandToExec := exec.Command(path+"/"+tokens[0], tokens[1:]...)
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
				fmt.Println(s + ": command not found")
			}
		}
	}
}
