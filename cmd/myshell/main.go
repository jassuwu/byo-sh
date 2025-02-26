package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

// Global list of built-in commands.
var BUILTIN_COMMANDS = []string{"echo", "exit", "type", "pwd", "cd"}

// autocomplete implements a simple autocompletion for builtins.
type autocomplete struct {
	input string
}

func NewAutocomplete(input string) autocomplete {
	return autocomplete{input: input}
}

// Completion returns a completion for the input if it is a prefix of a builtin.
func (a *autocomplete) Completion() string {
	for _, cmd := range BUILTIN_COMMANDS {
		if strings.HasPrefix(cmd, a.input) {
			return cmd
		}
	}
	return ""
}

// readInput reads user input interactively in raw mode, handling each keystroke.
// It supports backspace and autocompletion for "echo" and "exit" when TAB is pressed.
func readInput(prompt string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	var input []byte
	fmt.Print(prompt)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return "", fmt.Errorf("Error reading user input: %s", err)
		}
		if b == '\n' {
			fmt.Print("\r\n")
			break
		} else if b == '\t' {
			auto := NewAutocomplete(string(input))
			completion := auto.Completion()
			if completion != "" {
				input = []byte(completion)
			}
			// Reprint prompt and completed input.
			fmt.Print("\r\033[K" + prompt + string(input) + " ")
		} else if b == 127 || b == 8 {
			if len(input) > 0 {
				input = input[:len(input)-1]
			}
			fmt.Print("\r\033[K" + prompt + string(input))
		} else {
			input = append(input, b)
			fmt.Printf("%c", b)
		}
	}
	return strings.TrimSpace(string(input)), nil
}

// tokenize splits the input string into tokens using Bash‑style quoting rules.
func tokenize(input string) []string {
	var tokens []string
	var currentToken strings.Builder
	inSingleQuotes, inDoubleQuotes := false, false
	escapeNext := false

	for i := 0; i < len(input); i++ {
		c := input[i]
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
		if c == '\'' {
			if inDoubleQuotes {
				currentToken.WriteByte(c)
			} else {
				inSingleQuotes = true
			}
			continue
		}
		if c == '"' && !inSingleQuotes {
			inDoubleQuotes = !inDoubleQuotes
			continue
		}
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
	PATH := os.Getenv("PATH")
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error setting raw mode:", err)
		return
	}
	defer term.Restore(fd, oldState)

	for {
		// Read input with a prompt that clears the line.
		input, err := readInput("\r\033[K$ ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input:", err)
			continue
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		tokens := tokenize(input)
		if len(tokens) == 0 {
			continue
		}

		// Process redirection tokens.
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
					continue
				}
				stdoutFileName = tokens[i+1]
				stdoutAppend = (t == ">>" || t == "1>>")
				tokens = append(tokens[:i], tokens[i+2:]...)
				continue
			} else if t == "2>" || t == "2>>" {
				if i+1 >= len(tokens) {
					fmt.Fprintln(os.Stderr, "Redirection operator provided without filename")
					continue
				}
				stderrFileName = tokens[i+1]
				stderrAppend = (t == "2>>")
				tokens = append(tokens[:i], tokens[i+2:]...)
				continue
			}
			i++
		}

		var outWriter io.Writer = os.Stdout
		var errWriter io.Writer = os.Stderr
		var stdoutFile *os.File
		var stderrFile *os.File

		if stdoutFileName != "" {
			if stdoutAppend {
				stdoutFile, err = os.OpenFile(
					stdoutFileName,
					os.O_WRONLY|os.O_CREATE|os.O_APPEND,
					0644,
				)
			} else {
				stdoutFile, err = os.Create(stdoutFileName)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error opening file for stdout redirection:", err)
				continue
			}
			outWriter = stdoutFile
		}

		if stderrFileName != "" {
			if stderrAppend {
				stderrFile, err = os.OpenFile(
					stderrFileName,
					os.O_WRONLY|os.O_CREATE|os.O_APPEND,
					0644,
				)
			} else {
				stderrFile, err = os.Create(stderrFileName)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error opening file for stderr redirection:", err)
				if stdoutFile != nil {
					stdoutFile.Close()
				}
				continue
			}
			errWriter = stderrFile
		}

		// Execute the command.
		switch tokens[0] {
		case "exit":
			return
		case "echo":
			fmt.Fprintln(outWriter, strings.Join(tokens[1:], " "))
		case "type":
			if len(tokens) < 2 {
				fmt.Fprintln(errWriter, "type: missing argument")
				break
			}
			cmdName := tokens[1]
			found := false
			for _, b := range BUILTIN_COMMANDS {
				if b == cmdName {
					fmt.Fprintf(outWriter, "%s is a shell builtin\n\r", cmdName)
					found = true
					break
				}
			}
			if !found {
				paths := strings.Split(PATH, ":")
			TYPEPATHLOOP:
				for _, p := range paths {
					files, _ := os.ReadDir(p)
					for _, f := range files {
						if !f.IsDir() && f.Name() == cmdName {
							fmt.Fprintf(outWriter, "%s is %s/%s\n\r", cmdName, p, cmdName)
							found = true
							break TYPEPATHLOOP
						}
					}
				}
			}
			if !found {
				fmt.Fprintf(outWriter, "%s: not found\n\r", cmdName)
			}
		case "pwd":
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintln(errWriter, err)
			}
			fmt.Fprintf(outWriter, "%s\n\r", cwd)
		case "cd":
			if len(tokens) < 2 {
				fmt.Fprintln(errWriter, "cd: missing argument")
				break
			}
			newWD := tokens[1]
			if newWD == "~" {
				newWD = os.Getenv("HOME")
			}
			err := os.Chdir(newWD)
			if err != nil {
				fmt.Fprintf(errWriter, "cd: %s: No such file or directory\n\r", newWD)
			}
		default:
			found := false
			paths := strings.Split(PATH, ":")
		PATHLOOP:
			for _, p := range paths {
				files, _ := os.ReadDir(p)
				for _, f := range files {
					if !f.IsDir() && f.Name() == tokens[0] {
						cmd := exec.Command(p+"/"+tokens[0], tokens[1:]...)
						cmd.Args[0] = tokens[0] // Override Arg[0]

						var stderrBuf bytes.Buffer
						cmd.Stdout = outWriter
						cmd.Stdin = os.Stdin
						cmd.Stderr = &stderrBuf

						err := cmd.Run()
						// Trim leading whitespace from stderr and write to errWriter
						if err != nil {
							trimmedErr := strings.TrimLeft(stderrBuf.String(), " \t\n\r")
							fmt.Fprint(errWriter, trimmedErr)
						}

						found = true
						break PATHLOOP
					}
				}
			}
			if !found {
				fmt.Fprintf(outWriter, "%s: command not found\n\r", tokens[0])
			}
		}

		// Force a newline and flush stdout so the prompt appears at column 0.
		// fmt.Print("\r\n")
		os.Stdout.Sync()
		if stdoutFile != nil {
			stdoutFile.Close()
		}
		if stderrFile != nil {
			stderrFile.Close()
		}
	}
}
