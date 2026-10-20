package cli

import (
	"bytes"
	"os"
	"strings"
)

// wranglerTryCreate runs wrangler; treats "already exists" style errors as success.
func wranglerTryCreate(args ...string) error {
	out, err := wranglerCaptureBoth(args...)
	if err == nil {
		return nil
	}
	if wranglerErrorBenign(out) {
		return nil
	}
	os.Stderr.Write(out)
	return err
}

func wranglerCaptureQuiet(args ...string) (string, error) {
	out, err := wranglerCaptureBoth(args...)
	if err != nil && !wranglerErrorBenign(out) {
		return string(out), err
	}
	return string(out), nil
}

func wranglerCaptureBoth(args ...string) ([]byte, error) {
	cmd, err := wranglerCmd(args...)
	if err != nil {
		return nil, err
	}
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	err = cmd.Run()
	return combined.Bytes(), err
}

func wranglerErrorBenign(out []byte) bool {
	s := strings.ToLower(string(out))
	phrases := []string{
		"already exists",
		"already have a tunnel",
		"a project with this name already exists",
		"database with that name already exists",
		"bucket you tried to create already exists",
		"you own it",
	}
	for _, p := range phrases {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
