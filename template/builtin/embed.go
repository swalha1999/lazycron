package builtin

import "embed"

// FS holds the built-in templates: plain cron jobs (*.yaml) and Claude agent
// jobs (templates/ai/*.md) that `lazycron init --with-agents` copies into
// .lazycron/jobs/.
//
//go:embed templates/*
var FS embed.FS
