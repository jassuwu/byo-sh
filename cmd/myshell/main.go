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
// It treats text inside single quotes literally and in double quotes, backslashes
// only escape: $, `, ", \, or newline. Otherwise, characters are taken literally.
func tokenize(input string) []string {
	var tokens []string
	var currentToken strings.Builder
	inSingleQuotes, inDoubleQuotes := false, false
	escapeNext := false

	for i := 0; i < len(input); i++ {
		c := input[i]

		// Inside single quotes: take everything literally.
		if inSingleQuotes {
			if c == '\'' {
				inSingleQuotes = false
			} else {
				currentToken.WriteByte(c)
			}
			continue
		}

		if escapeNext {
			if inDoubleQuotes {
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

		// Single quotes: if inside double quotes, treat as literal.
		if c == '\'' {
			if inDoubleQuotes {
				currentToken.WriteByte(c)
			} else {
				inSingleQuotes = true
			}
			continue
		}

		// Toggle double quotes if not in single quotes.
		if c == '"' && !inSingleQuotes {
			inDoubleQuotes = !inDoubleQuotes
			continue
		}

		// Outside of double quotes, a space delimits tokens.
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

		// Process redirection tokens.
		// We'll support overwriting: ">" or "1>" for stdout, "2>" for stderr,
		// and appending: ">>" or "1>>" for stdout, "2>>" for stderr.
		var stdoutFileName string
		var stdoutAppend bool
		var stderrFileName string
		var stderrAppend bool

		i := 0
		for i < len(tokens) {
			t := tokens[i]
			if t == ">" || t == "1>" || t == ">>" || t == "1>>" {
				if i+1 >= len(tokens) {
					fmt.Fprintln(os.Stderr, "Redirection operator provided without filename")
					continue REPL
				}
				stdoutFileName = tokens[i+1]
				stdoutAppend = (t == ">>" || t == "1>>")
				// Remove the redirection operator and filename.
				tokens = append(tokens[:i], tokens[i+2:]...)
				continue // Check same index again.
			} else if t == "2>" || t == "2>>" {
				if i+1 >= len(tokens) {
					fmt.Fprintln(os.Stderr, "Redirection operator provided without filename")
					continue REPL
				}
				stderrFileName = tokens[i+1]
				stderrAppend = (t == "2>>")
				tokens = append(tokens[:i], tokens[i+2:]...)
				continue
			}
			i++
		}

		// Set default writers.
		var outWriter io.Writer = os.Stdout
		var errWriter io.Writer = os.Stderr
		var stdoutFile *os.File
		var stderrFile *os.File

		// Open stdout file if specified.
		if stdoutFileName != "" {
			if stdoutAppend {
				stdoutFile, err = os.OpenFile(stdoutFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
			} else {
				stdoutFile, err = os.Create(stdoutFileName)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error opening file for stdout redirection:", err)
				continue REPL
			}
			outWriter = stdoutFile
		}

		// Open stderr file if specified.
		if stderrFileName != "" {
			if stderrAppend {
				stderrFile, err = os.OpenFile(stderrFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
			} else {
				stderrFile, err = os.Create(stderrFileName)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error opening file for stderr redirection:", err)
				if stdoutFile != nil {
					stdoutFile.Close()
				}
				continue REPL
			}
			errWriter = stderrFile
		}

		// Execute command.
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
				fmt.Fprintln(errWriter, err)
			}
			fmt.Fprintln(outWriter, cwd)
		case "cd":
			newWD := tokens[1]
			if newWD == "~" {
				newWD = os.Getenv("HOME")
			}
			err := os.Chdir(newWD)
			if err != nil {
				fmt.Fprintln(errWriter, "cd:", newWD+":", "No such file or directory")
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
						// Override Arg[0] to display only the command name.
						commandToExec.Args[0] = tokens[0]
						commandToExec.Stdout = outWriter
						commandToExec.Stdin = os.Stdin
						commandToExec.Stderr = errWriter
						_ = commandToExec.Run() // Suppress extra error messages.
						found = true
						break PATHLOOP
					}
				}
			}
			if !found {
				fmt.Fprintln(outWriter, s+": command not found")
			}
		}

		if stdoutFile != nil {
			stdoutFile.Close()
		}
		if stderrFile != nil {
			stderrFile.Close()
		}
	}
}
