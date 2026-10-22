package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/ghcli"
	"github.com/irgipaulius/cogs/pkg/githubauth"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func cmdGitHub(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cogs github login|status|logout")
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, _, err := workspace.FindRoot(wd)
	if err != nil {
		return err
	}
	switch args[0] {
	case "login":
		return githubLogin(root, args[1:])
	case "status":
		return githubStatus(root)
	case "logout":
		return githubLogout(root)
	default:
		return fmt.Errorf("usage: cogs github login|status|logout")
	}
}

func githubLogout(root string) error {
	_ = os.Remove(root + "/.cogs/github.json")
	if bin, err := exec.LookPath("gh"); err == nil {
		cmd := exec.Command(bin, "auth", "logout", "--hostname", "github.com")
		_ = cmd.Run()
	}
	fmt.Println("GitHub: logged out (host gh + legacy .cogs/github.json)")
	return nil
}

func githubStatus(root string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	st, _ := ghcli.Status(ctx)
	if st.LoggedIn {
		fmt.Printf("GitHub: signed in as %s (gh)\n", st.Login)
		return nil
	}
	c, err := githubauth.Load(root)
	if err != nil {
		return err
	}
	if c.Token != "" {
		who := c.Login
		if who == "" {
			who = "(legacy token in .cogs/github.json)"
		}
		fmt.Printf("GitHub: signed in as %s (legacy file)\n", who)
		return nil
	}
	if !st.Installed {
		fmt.Println("GitHub: gh CLI not installed")
	} else {
		fmt.Println("GitHub: not signed in")
	}
	fmt.Println("Run: gh auth login && gh auth setup-git")
	fmt.Println("Or:  cogs github login --token")
	return nil
}

func githubLogin(root string, args []string) error {
	useToken := false
	for _, a := range args {
		if a == "--token" {
			useToken = true
		}
	}
	if tok := strings.TrimSpace(os.Getenv("COGS_GITHUB_TOKEN")); tok != "" {
		c, err := githubauth.SaveToken(root, tok)
		if err != nil {
			return err
		}
		fmt.Printf("Saved token for %s\n", c.Login)
		return nil
	}
	if useToken {
		fmt.Print("GitHub token: ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return err
		}
		c, err := githubauth.SaveToken(root, strings.TrimSpace(line))
		if err != nil {
			return err
		}
		fmt.Printf("Saved token for %s\n", c.Login)
		return nil
	}
	if _, err := exec.LookPath("gh"); err == nil {
		fmt.Println("Starting gh auth login (device flow)…")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if err := ghcli.LoginDevice(ctx); err != nil {
			return err
		}
		st, _ := ghcli.Status(ctx)
		fmt.Printf("Signed in as %s\n", st.Login)
		return nil
	}
	return fmt.Errorf("install GitHub CLI (gh) and run: gh auth login")
}
