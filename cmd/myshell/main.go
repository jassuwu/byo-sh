package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func tokenize(input string) []string {
	var tokens []string
	var currentToken strings.Builder
	inSingleQuotes, inDoubleQuotes := false, false
	escapeNext := false

	for i := 0; i < len(input); i++ {
		c := input[i]

		if escapeNext {
			// When an escape is active, process the next character:
			if inDoubleQuotes {
				// In double quotes, only a limited set of characters are escaped.
				if c == '$' || c == '`' || c == '"' || c == '\\' || c == '\n' {
					currentToken.WriteByte(c)
				} else {
					// For any other character, the backslash is preserved.
					currentToken.WriteByte('\\')
					currentToken.WriteByte(c)
				}
			} else {
				// Outside quotes (or in single quotes), the backslash always escapes the next character.
				currentToken.WriteByte(c)
			}
			escapeNext = false
			continue
		}

		// If we see a backslash, set the escape flag and skip this character.
		if c == '\\' {
			escapeNext = true
			continue
		}

		// Toggle single quotes if not in double quotes.
		if c == '\'' && !inDoubleQuotes {
			inSingleQuotes = !inSingleQuotes
			// Do not include the quote character itself in the token.
			continue
		}

		// Toggle double quotes if not in single quotes.
		if c == '"' && !inSingleQuotes {
			inDoubleQuotes = !inDoubleQuotes
			// Do not include the quote character itself in the token.
			continue
		}

		// If not inside any quotes, a space is a delimiter.
		if !inSingleQuotes && !inDoubleQuotes && c == ' ' {
			if currentToken.Len() > 0 {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
		} else {
			// Otherwise, append the character.
			currentToken.WriteByte(c)
		}
	}

	// Add the final token if any.
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
			TYPEPATHLOOP:
				for _, path := range paths {
					dirEntries, _ := os.ReadDir(path)
					for _, commandInPath := range dirEntries {
						if !commandInPath.IsDir() && commandToFindType == commandInPath.Name() {
							fmt.Println(commandToFindType, "is", path+"/"+commandToFindType)
							found = true
							break TYPEPATHLOOP
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
						// Create the command with the full path.
						commandToExec := exec.Command(path+"/"+tokens[0], tokens[1:]...)
						// Override Arg[0] with the original command name.
						commandToExec.Args[0] = tokens[0]
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
