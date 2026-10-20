package observe

import "strings"

// Mode controls observability: off, file (logs), dashboard (logs + local /admin UI).
type Mode string

const (
	ModeOff       Mode = "off"
	ModeFile      Mode = "file"
	ModeDashboard Mode = "dashboard"
)

func ParseMode(s string) Mode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "off", "none", "0":
		return ModeOff
	case "dashboard", "admin", "full":
		return ModeDashboard
	default:
		return ModeFile
	}
}
