package githubauth

import (
	"context"
	"time"

	"github.com/irgipaulius/cogs/pkg/ghcli"
)

func tokenFromGHCLI() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return ghcli.Token(ctx)
}
