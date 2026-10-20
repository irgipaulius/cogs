package cli

type serveMode int

const (
	serveForeground serveMode = iota
	serveBackground
	serveSkip
)
