package cli

import (
	"os"
	"strings"

	"github.com/irgipaulius/cogs/internal/cli/ux"
)

// GlobalOpts apply to every subcommand (agent-friendly).
type GlobalOpts struct {
	JSON    bool
	DryRun  bool
	Quiet   bool
	Verbose bool
	NoColor bool
	Yes     bool // -y / --yes: never prompt
}

var globals GlobalOpts

func peelGlobalFlags(args []string) []string {
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json" || a == "-json":
			globals.JSON = true
		case a == "--dry-run" || a == "-n":
			globals.DryRun = true
		case a == "--quiet" || a == "-q":
			globals.Quiet = true
		case a == "--verbose" || a == "-v":
			globals.Verbose = true
		case a == "--no-color":
			globals.NoColor = true
		case a == "-y", a == "--yes":
			globals.Yes = true
		case strings.HasPrefix(a, "--app="):
			// consumed by subcommands that scan args; keep in rest
			rest = append(rest, a)
		case a == "--app" && i+1 < len(args):
			rest = append(rest, a, args[i+1])
			i++
		default:
			rest = append(rest, a)
		}
	}
	color := !globals.NoColor
	if os.Getenv("COGS_JSON") == "1" {
		globals.JSON = true
	}
	ux.SetMode(ux.Mode{JSON: globals.JSON, Quiet: globals.Quiet, Verbose: globals.Verbose, Color: color})
	return rest
}
