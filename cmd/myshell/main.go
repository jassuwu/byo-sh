package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// tokenize splits the input string into tokens using Bash‑style quoting rules.
// It treats text inside single quotes literally, and inside double quotes,
// backslashes only escape: $, `, ", \, or newline. Otherwise, characters are taken literally.
func tokenize(input string) []string {
	var tokens []string
	var currentToken strings.Builder
	inSingleQuotes, inDoubleQuotes := false, false
	escapeNext := false

	for i := 0; i < len(input); i++ {
		c := input[i]

		// If we're inside single quotes, every character is literal.
		if inSingleQuotes {
			if c == '\'' { // closing single quote
				inSingleQuotes = false
			} else {
				currentToken.WriteByte(c)
			}
			continue
		}

		if escapeNext {
			if inDoubleQuotes {
				// In double quotes, only a limited set is escaped.
				if c == '$' || c == '`' || c == '"' || c == '\\' || c == '\n' {
					currentToken.WriteByte(c)
				} else {
					currentToken.WriteByte('\\')
					currentToken.WriteByte(c)
				}
			} else {
				currentToken.WriteByte(c)
			}
			escapeNext = false
			continue
		}

		if c == '\\' {
			escapeNext = true
			continue
		}

		// When encountering a single quote:
		// If inside double quotes, it should be taken literally.
		if c == '\'' {
			if inDoubleQuotes {
				currentToken.WriteByte(c)
			} else {
				inSingleQuotes = !inSingleQuotes
			}
			continue
		}

		// Toggle double quotes if not in single quotes.
		if c == '"' && !inSingleQuotes {
			inDoubleQuotes = !inDoubleQuotes
			continue
		}

		// Outside any quotes, a space is a delimiter.
		if !inDoubleQuotes && c == ' ' {
			if currentToken.Len() > 0 {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
			continue
		}

		currentToken.WriteByte(c)
	}

	if currentToken.Len() > 0 {
		tokens = append(tokens, currentToken.String())
	}
	return tokens
}

func main() {
	builtins := []string{"exit", "echo", "type", "pwd", "cd"}
	PATH := os.Getenv("PATH")
	reader := bufio.NewReader(os.Stdin)

REPL:
	for {
		fmt.Fprint(os.Stdout, "$ ")
		commandWithNewLine, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input:", err)
			continue
		}
		s := strings.Trim(commandWithNewLine, "\r\n")
		if s == "" {
			continue
		}
		tokens := tokenize(s)
		if len(tokens) == 0 {
			continue
		}

		// Check for output redirection: ">" or "1>"
		redir := false
		redirIndex := -1
		for i, t := range tokens {
			if t == ">" || t == "1>" {
				redir = true
				redirIndex = i
				break
			}
		}
		var outWriter io.Writer = os.Stdout
		var fileHandle *os.File
		if redir {
			if redirIndex+1 >= len(tokens) {
				fmt.Fprintln(os.Stderr, "Redirection operator provided without filename")
				continue REPL
			}
			fileName := tokens[redirIndex+1]
			fileHandle, err = os.Create(fileName)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error opening file for redirection:", err)
				continue REPL
			}
			outWriter = fileHandle
			// Remove redirection tokens from the command.
			tokens = tokens[:redirIndex]
			if len(tokens) == 0 {
				fileHandle.Close()
				continue REPL
			}
		}

		switch tokens[0] {
		case "exit":
			break REPL
		case "echo":
			fmt.Fprintln(outWriter, strings.Join(tokens[1:], " "))
		case "type":
			commandToFindType, found := tokens[1], false
			for _, builtin := range builtins {
				if builtin == commandToFindType {
					fmt.Fprintln(outWriter, commandToFindType, "is a shell builtin")
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
							fmt.Fprintln(outWriter, commandToFindType, "is", path+"/"+commandToFindType)
							found = true
							break TYPEPATHLOOP
						}
					}
				}
			}
			if !found {
				fmt.Fprintln(outWriter, commandToFindType+": not found")
			}
		case "pwd":
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			fmt.Fprintln(outWriter, cwd)
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
						// Override Arg[0] so the program sees only the command name.
						commandToExec.Args[0] = tokens[0]
						commandToExec.Stdout = outWriter
						commandToExec.Stdin = os.Stdin
						commandToExec.Stderr = os.Stderr
						_ = commandToExec.Run() // Ignore error output to prevent extra messages.
						found = true
						break PATHLOOP
					}
				}
			}
			if !found {
				fmt.Fprintln(outWriter, s+": command not found")
			}
		}
		if fileHandle != nil {
			fileHandle.Close()
		}
	}
}
