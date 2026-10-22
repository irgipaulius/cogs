package cli

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/irgipaulius/cogs/cogstemplate"
)

func cmdNew(args []string) error {
	fsFlags := flag.NewFlagSet("new", flag.ExitOnError)
	module := fsFlags.String("module", "", "Go module path (default github.com/irgipaulius/<project>)")
	with := fsFlags.String("with", "", "comma-separated features: geo,jobs")
	_ = fsFlags.Parse(args)
	rest := fsFlags.Args()
	if len(rest) < 1 {
		return fmt.Errorf("usage: cogs new <project> [--module path]")
	}
	nameArg := rest[0]
	dest := nameArg
	if !filepath.IsAbs(nameArg) {
		dest = filepath.Join(".", nameArg)
	}
	name := filepath.Base(dest)
	mod := *module
	if mod == "" {
		mod = "github.com/irgipaulius/" + name
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("directory %q already exists", dest)
	}
	cogsRoot, err := findCogsModuleRoot()
	if err != nil {
		return err
	}
	data := map[string]string{
		"Module":      mod,
		"Project":     name,
		"WithGeo":     fmt.Sprintf("%v", strings.Contains(*with, "geo")),
		"CogsReplace": cogsRoot,
	}
	if err := renderTemplate(dest, data); err != nil {
		return err
	}
	gomod := fmt.Sprintf("module %s\n\ngo 1.23\n\nrequire github.com/pressly/goose/v3 v3.24.2\n", mod)
	if err := os.WriteFile(filepath.Join(dest, "go.mod"), []byte(gomod), 0o644); err != nil {
		return err
	}
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dest
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}
	git := exec.Command("git", "init")
	git.Dir = dest
	_ = git.Run()
	fmt.Printf("created %s (module %s)\n", dest, mod)
	fmt.Printf("CoGS module root (for local dev): %s — use go.work, not replace directives in go.mod\n", cogsRoot)
	return nil
}

func findCogsModuleRoot() (string, error) {
	if p := os.Getenv("COGS_MODULE_PATH"); p != "" {
		return filepath.Abs(p)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			b, _ := os.ReadFile(filepath.Join(dir, "go.mod"))
			if strings.Contains(string(b), "module github.com/irgipaulius/cogs") {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("set COGS_MODULE_PATH to your checkout of github.com/irgipaulius/cogs")
}

func stripBuildIgnore(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "//go:build ignore" || t == "// +build ignore" {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func renderTemplate(dest string, data map[string]string) error {
	return fs.WalkDir(cogstemplate.FS, "template", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("template", path)
		if rel == "." {
			return nil
		}
		outPath := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(outPath, 0o755)
		}
		raw, err := fs.ReadFile(cogstemplate.FS, path)
		if err != nil {
			return err
		}
		content := string(raw)
		if strings.Contains(content, "{{") {
			tmpl, err := template.New(filepath.Base(path)).Parse(content)
			if err != nil {
				return err
			}
			var b strings.Builder
			if err := tmpl.Execute(&b, data); err != nil {
				return err
			}
			content = b.String()
		}
		content = stripBuildIgnore(content)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(outPath, []byte(content), 0o644)
	})
}
