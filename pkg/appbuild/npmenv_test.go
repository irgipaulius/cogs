package appbuild

import (
	"strings"
	"testing"
)

func TestNpmEnvStripsDeprecatedPythonGlobal(t *testing.T) {
	t.Setenv("npm_config_python", "/bad/global")
	env := NpmEnv(nil)
	for _, e := range env {
		if strings.HasPrefix(e, "npm_config_python=/bad") {
			t.Fatalf("kept deprecated global python: %s", e)
		}
	}
	foundGlobal := false
	for _, e := range env {
		if strings.HasPrefix(e, "NPM_CONFIG_GLOBALCONFIG=") {
			foundGlobal = true
		}
	}
	if !foundGlobal {
		t.Fatal("missing NPM_CONFIG_GLOBALCONFIG override")
	}
}
