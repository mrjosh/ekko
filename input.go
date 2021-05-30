package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

func NewInput(name, description string, required bool) (string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("[%s] %s", color.YellowString("Authentication"), description)

	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if required {
		if value != "" {
			return strings.TrimSpace(value), nil
		}
		return "", errors.New(name + " is required")
	}
	return strings.TrimSpace(value), nil
}
