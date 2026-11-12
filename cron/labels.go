package cron

import (
	"fmt"
	"unicode"
)

// ValidateLabel checks a free-text job field (name, tag, tag color, project)
// before it is written into the crontab. A newline or carriage return would
// start a new crontab line, and cron ends the command at an unescaped %, so
// both (and other control characters) are rejected.
func ValidateLabel(field, value string) error {
	for _, r := range value {
		if r == '%' {
			return fmt.Errorf("%s %q must not contain %%: cron treats it as end of command", field, value)
		}
		if unicode.IsControl(r) {
			return fmt.Errorf("%s %q must not contain control characters such as newlines", field, value)
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
