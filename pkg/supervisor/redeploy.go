package supervisor

import (
	"context"
)

// Redeploy runs push frontend (when applicable) and deployBackend for one app.
func Redeploy(ctx context.Context, wsRoot, appName string) (detail string, err error) {
	return redeployApp(ctx, wsRoot, appName)
}
