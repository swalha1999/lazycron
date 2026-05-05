package builtin

import "embed"

//go:embed templates/*
var FS embed.FS

//go:embed sandcastle_lib/*
var SandcastleLibFS embed.FS
