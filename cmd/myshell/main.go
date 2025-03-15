package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/term"
)

var builtinCMDs = []string{
	"exit",
	"echo",
	"type",
	"pwd",
	"cd",
}

type CMD struct {
	Name       string
	Args       []string
	Stdout     io.Writer
	Stderr     io.Writer
	childFiles []*os.File
}

func main() {
	for {
		fmt.Fprint(os.Stdout, "\r$ ")
		input := readInput(os.Stdin)
		cmd, err := parseCMD(input)
		if err != nil {
			fmt.Println(err)
			continue
		}
		switch cmd.Name {
		case "exit":
			cmd.Exit()
		case "echo":
			cmd.Echo()
		case "type":
			cmd.Type()
		case "pwd":
			cmd.PWD()
		case "cd":
			cmd.CD()
		case "":
			continue
		default:
			command := exec.Command(cmd.Name, cmd.Args...)
			command.Stdout = cmd.Stdout
			command.Stderr = cmd.Stderr
			if err := command.Run(); err != nil {
				var execErr *exec.ExitError
				if errors.As(err, &execErr) {
					continue
				}
				fmt.Println(cmd.Name + ": command not found")
			}
		}
	}
}

func readInput(rd io.Reader) (input string) {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)
	r := bufio.NewReader(rd)
	wasTab := false
	autocompleteNames := []string{}
loop:
	for {
		c, _, err := r.ReadRune()
		if err != nil {
			fmt.Println(err)
			continue
		}
		switch c {
		case '\x03': // Ctrl+C
			os.Exit(0)
		case '\r', '\n': // Enter
			fmt.Fprint(os.Stdout, "\r\n")
			break loop
		case '\x7F': // Backspace
			if length := len(input); length > 0 {
				input = input[:length-1]
				fmt.Fprint(os.Stdout, "\b \b")
			}
			autocompleteNames = nil
			wasTab = false
		case '\t': // Tab
			if len(autocompleteNames) == 0 {
				names, found := autocomplete(input)
				if !found {
					fmt.Fprint(os.Stdout, "\a")
					continue
				}
				autocompleteNames = names
			}
			switch {
			case len(autocompleteNames) == 1:
				suffix := strings.TrimPrefix(autocompleteNames[0], input)
				input += suffix + " "
				fmt.Fprint(os.Stdout, suffix+" ")
			case len(autocompleteNames) > 1:
				longestCommonPrefix, found := findLongestCommonPrefix(autocompleteNames)
				if found {
					suffix := strings.TrimPrefix(longestCommonPrefix, input)
					input += suffix
					fmt.Fprint(os.Stdout, suffix)
					autocompleteNames = nil
					wasTab = false
					continue
				}
				if !wasTab {
					fmt.Fprint(os.Stdout, "\a")
					wasTab = true
					continue
				}
				fmt.Fprintf(os.Stdout, "\r\n%s\r\n", strings.Join(autocompleteNames, "  "))
				fmt.Fprint(os.Stdout, "$ ", input)
			}
		default:
			input += string(c)
			fmt.Fprint(os.Stdout, string(c))
			wasTab = false
			autocompleteNames = nil
		}
	}
	return
}

func parseCMD(s string) (*CMD, error) {
	cmd := CMD{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	sanitized := sanitizeInput(s)
	if len(sanitized) > 0 {
		cmd.Name = sanitized[0]
	}
	if len(sanitized) > 1 {
		cmd.Args = sanitized[1:]
	}
	for i, arg := range cmd.Args {
		if i+1 > len(cmd.Args) {
			continue
		}
		switch arg {
		case ">", "1>", "2>":
			f, err := os.OpenFile(cmd.Args[i+1], os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				return nil, err
			}
			switch arg {
			case ">", "1>":
				cmd.Stdout = f
			case "2>":
				cmd.Stderr = f
			}
			cmd.Args = cmd.Args[:i]
			cmd.childFiles = append(cmd.childFiles, f)
		case ">>", "1>>", "2>>":
			f, err := os.OpenFile(cmd.Args[i+1], os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
			if err != nil {
				return nil, err
			}
			switch arg {
			case ">>", "1>>":
				cmd.Stdout = f
			case "2>>":
				cmd.Stderr = f
			}
			cmd.Args = cmd.Args[:i]
			cmd.childFiles = append(cmd.childFiles, f)
		}
	}
	return &cmd, nil
}

func sanitizeInput(s string) (args []string) {
	var sb strings.Builder
	inSingleQuotes := false
	inDoubleQuotes := false
	escaped := false
	for i, c := range s {
		switch {
		case escaped:
			sb.WriteRune(c)
			escaped = false
		case c == '\'':
			if inDoubleQuotes {
				sb.WriteRune(c)
				continue
			}
			inSingleQuotes = !inSingleQuotes
		case c == '"':
			if inSingleQuotes {
				sb.WriteRune(c)
				continue
			}
			inDoubleQuotes = !inDoubleQuotes
		case c == '\\':
			switch {
			case inSingleQuotes:
				sb.WriteRune(c)
			case inDoubleQuotes:
				if i+1 >= len(s) {
					sb.WriteRune(c)
					continue
				}
				nextC := s[i+1]
				if nextC == '\\' || nextC == '$' || nextC == '"' {
					escaped = true
					continue
				}
				sb.WriteRune(c)
			default:
				escaped = true
			}
		case unicode.IsSpace(c):
			if inSingleQuotes || inDoubleQuotes {
				sb.WriteRune(c)
				continue
			}
			if sb.Len() > 0 {
				args = append(args, sb.String())
				sb.Reset()
			}
		default:
			sb.WriteRune(c)
		}
	}
	if sb.Len() > 0 {
		args = append(args, sb.String())
	}
	return
}

func (c *CMD) Exit() {
	if len(c.Args) == 0 {
		os.Exit(0)
	}
	code, err := strconv.Atoi(c.Args[0])
	if err != nil {
		fmt.Println("err convert exit code:", err.Error())
		os.Exit(0)
	}
	os.Exit(code)
}

func (c *CMD) Echo() {
	defer c.closeChildFiles()
	fmt.Fprintln(c.Stdout, strings.Join(c.Args, " "))
}

func (c *CMD) closeChildFiles() {
	for _, f := range c.childFiles {
		f.Close()
	}
}

func (c *CMD) Type() {
	if len(c.Args) == 0 {
		fmt.Println("missing argument")
		return
	}
	value := c.Args[0]
	if slices.Contains(builtinCMDs, value) {
		fmt.Println(value, "is a shell builtin")
		return
	}
	path, err := exec.LookPath(value)
	if err != nil {
		fmt.Println(value + ": not found")
		return
	}
	fmt.Println(value, "is", path)
}

func (c *CMD) PWD() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println(dir)
}

func (c *CMD) CD() {
	if len(c.Args) == 0 {
		return
	}
	dir := c.Args[0]
	if dir == "~" {
		dir = os.Getenv("HOME")
	}
	if err := os.Chdir(dir); err != nil {
		fmt.Printf("cd: %s: No such file or directory\n", dir)
	}
}

func autocomplete(prefix string) (names []string, found bool) {
	if prefix == "" {
		return
	}
	names = append(names, findBuiltinExecutablesHasPrefix(prefix)...)
	names = append(names, findExecutablesHasPrefix(prefix)...)
	names = removeDuplicates(names)
	slices.Sort(names)
	found = len(names) > 0
	return
}

func removeDuplicates(duplicates []string) (after []string) {
	dup := map[string]struct{}{}
	for _, v := range duplicates {
		if _, ok := dup[v]; !ok {
			dup[v] = struct{}{}
			after = append(after, v)
		}
	}
	return
}

func findBuiltinExecutablesHasPrefix(prefix string) (names []string) {
	for _, v := range builtinCMDs {
		if strings.HasPrefix(v, prefix) {
			names = append(names, v)
		}
	}
	return
}

func findExecutablesHasPrefix(prefix string) (names []string) {
	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			dir = "."
		}
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasPrefix(d.Name(), prefix) {
				return err
			}
			info, _ := d.Info()
			if info.Mode()&0111 != 0 {
				names = append(names, d.Name())
			}
			return nil
		})
	}
	return
}

func findLongestCommonPrefix(names []string) (longestCommonPrefix string, found bool) {
	if len(names) == 0 {
		return
	}
	slices.Sort(names)
	longestCommonPrefix = names[0]
	for _, v := range names[1:] {
		if !strings.HasPrefix(v, longestCommonPrefix) {
			return "", false
		}
	}
	return longestCommonPrefix, true
}
