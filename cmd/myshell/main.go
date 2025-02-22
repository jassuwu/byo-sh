package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
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

// readLine reads user input interactively in raw mode, handling each keypress.
// It supports backspace and auto‑completion for the builtins "echo" and "exit" when TAB is pressed.
func readLine(prompt string) (string, error) {
	// Print the prompt.
	fmt.Print(prompt)
	var buf []rune
	for {
		// Read one byte from stdin.
		var b [1]byte
		n, err := os.Stdin.Read(b[:])
		if err != nil || n == 0 {
			return "", err
		}
		c := b[0]
		if c == '\r' || c == '\n' {
			// When Enter is pressed, print a newline and return the current input.
			fmt.Print("\r\n")
			break
		} else if c == 9 { // TAB key
			// Auto-complete: if the current input is a prefix of "echo" or "exit"
			current := string(buf)
			if strings.HasPrefix("echo", current) {
				buf = []rune("echo ")
			} else if strings.HasPrefix("exit", current) {
				buf = []rune("exit ")
			}
			// Reprint the prompt and current buffer.
			fmt.Print("\r\033[K") // \r returns carriage, \033[K clears to end of line.
			fmt.Print(prompt + string(buf))
		} else if c == 127 || c == 8 { // Backspace key (127 or 8)
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
			}
			fmt.Print("\r\033[K")
			fmt.Print(prompt + string(buf))
		} else {
			// Append normal characters.
			buf = append(buf, rune(c))
			fmt.Printf("%c", c)
		}
	}
	return string(buf), nil
}

func main() {
	// Builtins: auto-completion will work only for "echo" and "exit".
	builtins := []string{"exit", "echo", "type", "pwd", "cd"}
	PATH := os.Getenv("PATH")

	// Put the terminal into raw mode for fully interactive key handling.
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error setting raw mode:", err)
		return
	}
	defer term.Restore(fd, oldState)

	// Main REPL loop.
	for {
		// Use our custom readLine function.
		s, err := readLine("\r\033[K$ ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading line:", err)
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}

		// Tokenize the input.
		tokens := tokenize(s)
		if len(tokens) == 0 {
			continue
		}

		// Process redirection tokens.
		// Supported redirections:
		//   stdout overwrite: ">" or "1>"
		//   stdout append:    ">>" or "1>>"
		//   stderr overwrite: "2>"
		//   stderr append:    "2>>"
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

		// Set default writers.
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

		// Execute command.
		switch tokens[0] {
		case "exit":
			return
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
					entries, _ := os.ReadDir(path)
					for _, entry := range entries {
						if !entry.IsDir() && commandToFindType == entry.Name() {
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
				entries, _ := os.ReadDir(path)
				for _, entry := range entries {
					if !entry.IsDir() && entry.Name() == tokens[0] {
						cmd := exec.Command(path+"/"+tokens[0], tokens[1:]...)
						// Override Arg[0] so the program sees only the command name.
						cmd.Args[0] = tokens[0]
						cmd.Stdout = outWriter
						cmd.Stdin = os.Stdin
						cmd.Stderr = errWriter
						_ = cmd.Run()
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
