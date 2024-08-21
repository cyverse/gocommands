package commons

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"
)

func Input(msg string) (string, error) {
	terminalWriter := GetTerminalWriter()

	terminalWriter.Lock()
	defer terminalWriter.Unlock()

	red := "\033[31m"
	reset := "\033[0m"

	fmt.Printf("%s%s: %s", red, msg, reset)

	userInput := ""
	_, err := fmt.Scanln(&userInput)

	return userInput, err
}

// InputYN inputs Y or N
// true for Y, false for N
func InputYN(msg string) bool {
	for {
		inputString, _ := Input(fmt.Sprintf("%s [y/n]", msg))
		inputString = strings.ToLower(inputString)
		if inputString == "y" || inputString == "yes" || inputString == "true" {
			return true
		} else if inputString == "n" || inputString == "no" || inputString == "false" {
			return false
		}
	}
}

func InputInt(msg string) (int, error) {
	inputString, err := Input(msg)
	if err != nil {
		return -1, err
	}

	return strconv.Atoi(inputString)
}

func InputPassword(msg string) (string, error) {
	terminalWriter := GetTerminalWriter()

	terminalWriter.Lock()
	defer terminalWriter.Unlock()

	red := "\033[31m"
	reset := "\033[0m"

	fmt.Printf("%s%s: %s", red, msg, reset)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Print("\n")

	if err != nil {
		return "", err
	}

	return string(bytePassword), nil
}
