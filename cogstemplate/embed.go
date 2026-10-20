// Package cogstemplate embeds the `cogs new` scaffold.
package cogstemplate

import "embed"

//go:embed all:template
var FS embed.FS
