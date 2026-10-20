package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/irgipaulius/cogs/internal/cli/ux"
	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/workspace"
	"golang.org/x/term"
)

func cmdAdmin(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cogs admin init|check")
	}
	switch args[0] {
	case "init":
		return cmdAdminInit(args[1:])
	case "check":
		return cmdAdminCheck(args[1:])
	default:
		return fmt.Errorf("unknown admin subcommand %q", args[0])
	}
}

func cmdAdminInit(args []string) error {
	fs := flag.NewFlagSet("admin init", flag.ContinueOnError)
	dir := fs.String("dir", "apps/admin", "admin app directory")
	username := fs.String("username", "", "root username (non-interactive)")
	password := fs.String("password", "", "root password (non-interactive, min 10 chars)")
	_ = fs.Parse(args)
	adminDir, err := resolveAdminDir(*dir)
	if err != nil {
		return err
	}
	dbPath := filepath.Join(adminDir, "data", "admin.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		return err
	}
	store, err := adminstore.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	ok, err := store.HasAnyUser(context.Background())
	if err != nil {
		return err
	}
	if ok {
		return fmt.Errorf("admin already initialized at %s", dbPath)
	}
	user := strings.TrimSpace(*username)
	pass := *password
	if user == "" || pass == "" {
		user, err = askString("Root username", "")
		if err != nil {
			return err
		}
		pass, err = readPassword("Root password (min 10 chars): ")
		if err != nil {
			return err
		}
		pass2, err := readPassword("Confirm password: ")
		if err != nil {
			return err
		}
		if pass != pass2 {
			return fmt.Errorf("passwords do not match")
		}
	}
	if err := store.CreateRootUser(context.Background(), user, pass); err != nil {
		return err
	}
	ux.OK("admin root user created — " + dbPath)
	return nil
}

func resolveAdminDir(dir string) (string, error) {
	wd, _ := os.Getwd()
	adminDir := dir
	if !filepath.IsAbs(adminDir) {
		if root, _, err := workspace.FindRoot(wd); err == nil {
			adminDir = filepath.Join(root, adminDir)
		} else {
			adminDir = filepath.Join(wd, adminDir)
		}
	}
	return adminDir, nil
}

func cmdAdminCheck(args []string) error {
	fs := flag.NewFlagSet("admin check", flag.ContinueOnError)
	dir := fs.String("dir", "apps/admin", "admin app directory")
	_ = fs.Parse(args)
	adminDir, err := resolveAdminDir(*dir)
	if err != nil {
		return err
	}
	dbPath := filepath.Join(adminDir, "data", "admin.db")
	store, err := adminstore.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	ok, err := store.HasAnyUser(context.Background())
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("admin not initialized — run: cogs admin init")
	}
	ux.OK("admin ready")
	return nil
}

func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	b, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ensureAdminInitialized(adminDir string) error {
	dbPath := filepath.Join(adminDir, "data", "admin.db")
	store, err := adminstore.Open(dbPath)
	if err != nil {
		return fmt.Errorf("admin db: %w", err)
	}
	defer store.Close()
	ok, err := store.HasAnyUser(context.Background())
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("admin has no users — run: cogs admin init (required before public deploy)")
	}
	return nil
}
