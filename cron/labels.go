package cron

import (
	"fmt"
	"strings"
	"unicode"
)

// reservedInLabel is, per field, the text the name comment
// ("# name @id:x @runner:r @once [tag:color] {project}") uses as delimiters.
// A label holding it would be read back as a different name, id, tag or
// project the next time the crontab is parsed.
var reservedInLabel = map[string][]string{
	"name":      {" @id:", " @runner:"},
	"tag":       {"[", "]"},
	"tag color": {"[", "]", ":"},
	"project":   {"{", "}"},
}

// ValidateLabel checks a free-text job field (name, tag, tag color, project)
// before it is written into the crontab. A newline or carriage return would
// start a new crontab line, and cron ends the command at an unescaped %, so
// both (and other control characters) are rejected, as is the field's
// reserved delimiter text.
func ValidateLabel(field, value string) error {
	for _, r := range value {
		if r == '%' {
			return fmt.Errorf("%s %q must not contain %%: cron treats it as end of command", field, value)
		}
		if unicode.IsControl(r) {
			return fmt.Errorf("%s %q must not contain control characters such as newlines", field, value)
		}
	}
	for _, reserved := range reservedInLabel[field] {
		if strings.Contains(value, reserved) {
			return fmt.Errorf("%s %q must not contain %q: lazycron uses it to mark job fields in the crontab", field, value, strings.TrimSpace(reserved))
		}
	}
	return nil
}

// ValidateLabels checks a job's name, tag, tag color and project with
// ValidateLabel.
func (j Job) ValidateLabels() error {
	labels := []struct{ field, value string }{
		{"name", j.Name},
		{"tag", j.Tag},
		{"tag color", j.TagColor},
		{"project", j.Project},
	}
	for _, l := range labels {
		if err := ValidateLabel(l.field, l.value); err != nil {
			return err
		}
	}
	return nil
}
