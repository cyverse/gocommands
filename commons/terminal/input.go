package terminal

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/term"
)

var (
	selectedYesAll bool = false
	selectedNoAll  bool = false
	inputMutex     sync.Mutex
)

func Input(msg string) string {
	inputMutex.Lock()
	defer inputMutex.Unlock()

	return inputLocked(msg)
}

func inputLocked(msg string) string {
	terminalWriter := GetTerminalWriter()

	terminalWriter.Lock()
	red := "\033[31m"
	reset := "\033[0m"
	fmt.Printf("%s%s: %s", red, msg, reset)
	terminalWriter.Unlock()

	userInput := ""
	fmt.Scanln(&userInput)

	return userInput
}

// InputYN inputs Y or N
// true for Y, false for N
func InputYN(msg string) bool {
	inputMutex.Lock()
	defer inputMutex.Unlock()

	if selectedYesAll {
		return true
	}

	if selectedNoAll {
		return false
	}

	for {
		inputString := inputLocked(fmt.Sprintf("%s [yes(y)/no(n)/yes-all(a)/no-all(na)]", msg))
		inputString = strings.ToLower(inputString)
		switch inputString {
		case "y", "yes", "true":
			return true
		case "n", "no", "false":
			return false
		case "a", "ya", "all", "yes-all":
			selectedYesAll = true
			return true
		case "na", "none", "no-all":
			selectedNoAll = true
			return false
		}
	}
}

func InputInt(msg string) int {
	inputString := Input(msg)
	if len(inputString) == 0 {
		return 0
	}

	v, err := strconv.Atoi(inputString)
	if err != nil {
		return 0
	}

	return v
}

func InputPassword(msg string) (string, error) {
	inputMutex.Lock()
	defer inputMutex.Unlock()

	if !term.IsTerminal(int(syscall.Stdin)) {
		return "", fmt.Errorf("password input requires an interactive terminal")
	}

	terminalWriter := GetTerminalWriter()

	terminalWriter.Lock()
	red := "\033[31m"
	reset := "\033[0m"
	fmt.Printf("%s%s: %s", red, msg, reset)
	terminalWriter.Unlock()

	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Print("\n")

	if err != nil {
		return "", err
	}

	return string(bytePassword), nil
}
