package ghcli

import (
	"context"
	"strings"
)

// StarRepoBestEffort stars owner/repo using the host gh login. Errors are ignored.
func StarRepoBestEffort(ctx context.Context, fullName string) {
	fullName = strings.TrimSpace(fullName)
	fullName = strings.TrimPrefix(fullName, "https://github.com/")
	fullName = strings.TrimSuffix(fullName, ".git")
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return
	}
	st, err := Status(ctx)
	if err != nil || !st.LoggedIn {
		return
	}
	endpoint := "user/starred/" + parts[0] + "/" + parts[1]
	_, _ = run(ctx, "api", "--method", "PUT", "-H", "Accept: application/vnd.github+json", endpoint)
}
