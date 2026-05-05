package builtin

import "embed"

//go:embed templates/*
var FS embed.FS

//go:embed sandcastle_lib/*
var SandcastleLibFS embed.FS

// SandcastleInitFS holds the Dockerfile, .env.example, and .gitignore that
// lazycron writes directly into a freshly-scaffolded .sandcastle/ — replacing
// the interactive `sandcastle init` flow. Defaults: Claude Code agent, Docker
// sandbox, GitHub Issues backlog manager. Sourced verbatim from
// @ai-hero/sandcastle@0.5.7's InitService.
//
//go:embed sandcastle_init/*
var SandcastleInitFS embed.FS
