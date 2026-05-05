package template

import (
	"fmt"
	"regexp"
)

// SandcastleMeta holds the metadata exported from a .sandcastle/jobs/*.ts file.
// The fields are populated by static parse — no TS execution.
type SandcastleMeta struct {
	Cron     string
	Name     string
	Tag      string
	TagColor string
}

// exportRegex matches `export const <ident> = "<string>"` at the start of a line,
// with double-quoted string literals only. Backticks, single quotes, and any
// computed expression are intentionally not recognised.
var exportRegex = regexp.MustCompile(`(?m)^export\s+const\s+(\w+)\s*=\s*"((?:[^"\\]|\\.)*)"\s*;?\s*$`)

// ParseSandcastleMeta extracts metadata from a sandcastle job TS file.
// Returns an error if `cron` or `name` is missing.
func ParseSandcastleMeta(content []byte) (SandcastleMeta, error) {
	m := SandcastleMeta{}
	for _, match := range exportRegex.FindAllStringSubmatch(string(content), -1) {
		switch match[1] {
		case "cron":
			m.Cron = unescape(match[2])
		case "name":
			m.Name = unescape(match[2])
		case "tag":
			m.Tag = unescape(match[2])
		case "tagColor":
			m.TagColor = unescape(match[2])
		}
	}
	if m.Cron == "" {
		return m, fmt.Errorf("missing 'export const cron'")
	}
	if m.Name == "" {
		return m, fmt.Errorf("missing 'export const name'")
	}
	return m, nil
}

// unescape converts the limited set of escapes we accept inside double-quoted
// string literals (\" and \\) back to their literal forms.
func unescape(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			out = append(out, s[i])
			continue
		}
		out = append(out, s[i])
	}
	return string(out)
}
