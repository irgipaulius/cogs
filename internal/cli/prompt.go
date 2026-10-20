package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func askString(prompt, defaultVal string) (string, error) {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultVal)
	} else {
		fmt.Printf("%s: ", prompt)
	}
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal, nil
	}
	return line, nil
}

func askYesNo(prompt string, defaultYes bool) (bool, error) {
	def := "y/N"
	if defaultYes {
		def = "Y/n"
	}
	fmt.Printf("%s (%s): ", prompt, def)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return false, err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return defaultYes, nil
	}
	return line == "y" || line == "yes", nil
}
