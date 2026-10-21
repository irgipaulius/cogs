package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
		return os.Remove(githubauthPath(root))
	default:
		return fmt.Errorf("usage: cogs github login|status|logout")
	}
}

func githubauthPath(root string) string {
	return root + "/.cogs/github.json"
}

func githubStatus(root string) error {
	c, err := githubauth.Load(root)
	if err != nil {
		return err
	}
	if c.Token == "" {
		fmt.Println("GitHub: not signed in")
		fmt.Println("Run: cogs github login")
		return nil
	}
	who := c.Login
	if who == "" {
		who = "(unknown user)"
	}
	fmt.Printf("GitHub: signed in as %s\n", who)
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
	clientID, err := githubauth.ClientID()
	if err != nil {
		if globals.Yes {
			return fmt.Errorf("%w — or set COGS_GITHUB_TOKEN", err)
		}
		return err
	}
	start, err := githubauth.StartDeviceFlow(clientID)
	if err != nil {
		return err
	}
	fmt.Printf("Open %s\nEnter code: %s\n", start.VerificationURI, start.UserCode)
	c, err := githubauth.PollDeviceFlow(clientID, start.DeviceCode, start.Interval)
	if err != nil {
		return err
	}
	if err := githubauth.Save(root, c); err != nil {
		return err
	}
	fmt.Printf("Signed in as %s\n", c.Login)
	return nil
}

func requireGitHubToken(root string) (string, error) {
	tok := githubauth.Token(root)
	if tok != "" {
		return tok, nil
	}
	if globals.Yes {
		return "", fmt.Errorf("GitHub token required — run: cogs github login (or set COGS_GITHUB_TOKEN)")
	}
	return "", fmt.Errorf("run cogs github login first")
}
