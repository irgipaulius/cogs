package ux

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Mode controls human vs machine output.
type Mode struct {
	JSON    bool
	Quiet   bool
	Verbose bool
	Color   bool
}

var out Mode

// SetMode configures package-level output (called once per CLI invocation).
func SetMode(m Mode) {
	out = m
	if !m.Color || os.Getenv("NO_COLOR") != "" {
		out.Color = false
	} else {
		st, err := os.Stdout.Stat()
		out.Color = err == nil && (st.Mode()&os.ModeCharDevice) != 0
	}
}

func c(code, s string) string {
	if !out.Color {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

func Bold(s string) string  { return c("1", s) }
func Dim(s string) string   { return c("2", s) }
func Green(s string) string { return c("32", s) }
func Cyan(s string) string  { return c("36", s) }
func Yellow(s string) string { return c("33", s) }
func Red(s string) string   { return c("31", s) }

// Event is a single JSON log line for agents.
type Event struct {
	Level   string         `json:"level"`
	Step    string         `json:"step,omitempty"`
	Message string         `json:"message"`
	App     string         `json:"app,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

func emit(level, step, msg, app string, data map[string]any) {
	if out.JSON {
		e := Event{Level: level, Step: step, Message: msg, App: app, Data: data}
		b, _ := json.Marshal(e)
		fmt.Fprintln(os.Stdout, string(b))
		return
	}
	if out.Quiet && level != "error" {
		return
	}
	prefix := ""
	switch level {
	case "ok":
		prefix = Green("✓")
	case "warn":
		prefix = Yellow("!")
	case "error":
		prefix = Red("✗")
	case "step":
		prefix = Cyan("→")
	default:
		prefix = Dim("·")
	}
	line := prefix + " " + msg
	if app != "" {
		line = Dim("["+app+"]") + " " + line
	}
	fmt.Fprintln(os.Stdout, line)
}

func Info(msg string)    { emit("info", "", msg, "", nil) }
func Step(msg string)    { emit("step", "run", msg, "", nil) }
func OK(msg string)      { emit("ok", "", msg, "", nil) }
func Warn(msg string)    { emit("warn", "", msg, "", nil) }
func Fail(msg string)    { emit("error", "", msg, "", nil) }

func AppStep(app, msg string) { emit("step", "run", msg, app, nil) }
func AppOK(app, msg string)   { emit("ok", "", msg, app, nil) }

func Title(s string) {
	if out.JSON {
		emit("info", "", s, "", nil)
		return
	}
	fmt.Fprintln(os.Stdout, Bold(s))
}

func Plan(title string, lines []string) {
	if out.JSON {
		emit("info", "plan", title, "", map[string]any{"lines": lines})
		return
	}
	Title(title)
	for _, l := range lines {
		fmt.Fprintln(os.Stdout, "  "+Dim(l))
	}
}

func WriteError(w io.Writer, err error) {
	if out.JSON {
		b, _ := json.Marshal(Event{Level: "error", Message: err.Error()})
		fmt.Fprintln(w, string(b))
		return
	}
	fmt.Fprintf(w, "%s %v\n", Red("cogs:"), err)
}

func JoinFlags(flags map[string]string) string {
	var parts []string
	for k, v := range flags {
		if v == "true" {
			parts = append(parts, "--"+k)
		} else if v != "" && v != "false" {
			parts = append(parts, "--"+k+"="+v)
		}
	}
	return strings.Join(parts, " ")
}
