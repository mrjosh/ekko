package helpers

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

func EncodeBase64(obj interface{}) (string, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func DecodeBase64(in string, obj interface{}) error {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, obj)
	if err != nil {
		return err
	}
	return nil
}

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
