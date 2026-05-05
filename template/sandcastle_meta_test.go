package template

import (
	"strings"
	"testing"
)

func TestParseSandcastleMeta(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     SandcastleMeta
		wantErr  bool
		errMatch string
	}{
		{
			name: "happy path with all fields",
			input: `export const cron = "0 9 * * 1-5";
export const name = "Fix Agent";
export const tag = "BP";
export const tagColor = "#f38ba8";

import { run } from "@ai-hero/sandcastle";
`,
			want: SandcastleMeta{
				Cron:     "0 9 * * 1-5",
				Name:     "Fix Agent",
				Tag:      "BP",
				TagColor: "#f38ba8",
			},
		},
		{
			name: "minimum required fields only",
			input: `export const cron = "*/5 * * * *";
export const name = "Heartbeat";
`,
			want: SandcastleMeta{
				Cron: "*/5 * * * *",
				Name: "Heartbeat",
			},
		},
		{
			name: "no semicolons",
			input: `export const cron = "0 0 * * *"
export const name = "Daily"
`,
			want: SandcastleMeta{Cron: "0 0 * * *", Name: "Daily"},
		},
		{
			name:     "missing cron",
			input:    `export const name = "Foo";`,
			wantErr:  true,
			errMatch: "missing 'export const cron'",
		},
		{
			name:     "missing name",
			input:    `export const cron = "0 9 * * *";`,
			wantErr:  true,
			errMatch: "missing 'export const name'",
		},
		{
			name: "comments interspersed",
			input: `// Schedule: business hours weekdays
export const cron = "0 9 * * 1-5";
// Display name in lazycron list
export const name = "Fix Agent";
`,
			want: SandcastleMeta{Cron: "0 9 * * 1-5", Name: "Fix Agent"},
		},
		{
			name: "escaped quotes in name",
			input: `export const cron = "0 9 * * *";
export const name = "Agent \"Foo\"";
`,
			want: SandcastleMeta{Cron: "0 9 * * *", Name: `Agent "Foo"`},
		},
		{
			name: "ignored unknown export",
			input: `export const cron = "0 9 * * *";
export const name = "Agent";
export const description = "ignored field";
`,
			want: SandcastleMeta{Cron: "0 9 * * *", Name: "Agent"},
		},
		{
			name: "notify false",
			input: `export const cron = "0 9 * * *";
export const name = "Agent";
export const notify = false;
`,
			want: SandcastleMeta{Cron: "0 9 * * *", Name: "Agent", Notify: boolPtr(false)},
		},
		{
			name: "notify true",
			input: `export const cron = "0 9 * * *";
export const name = "Agent";
export const notify = true;
`,
			want: SandcastleMeta{Cron: "0 9 * * *", Name: "Agent", Notify: boolPtr(true)},
		},
		{
			name: "notify omitted leaves Notify nil",
			input: `export const cron = "0 9 * * *";
export const name = "Agent";
`,
			want: SandcastleMeta{Cron: "0 9 * * *", Name: "Agent"},
		},
		{
			name: "single-quoted is not recognised",
			input: `export const cron = '0 9 * * *';
export const name = "Agent";
`,
			wantErr:  true,
			errMatch: "missing 'export const cron'",
		},
		{
			name: "template literal is not recognised",
			input: "export const cron = `0 9 * * *`;\nexport const name = \"Agent\";\n",
			wantErr:  true,
			errMatch: "missing 'export const cron'",
		},
		{
			name: "export not at line start is not recognised",
			input: `// inline: export const cron = "0 9 * * *";
export const name = "Agent";
`,
			wantErr:  true,
			errMatch: "missing 'export const cron'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseSandcastleMeta([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tc.errMatch)
				}
				if !strings.Contains(err.Error(), tc.errMatch) {
					t.Fatalf("expected error containing %q, got %q", tc.errMatch, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !sandcastleMetaEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }

// sandcastleMetaEqual compares two SandcastleMeta values, including the
// Notify *bool pointer (nil-safe value equality).
func sandcastleMetaEqual(a, b SandcastleMeta) bool {
	if a.Cron != b.Cron || a.Name != b.Name || a.Tag != b.Tag || a.TagColor != b.TagColor {
		return false
	}
	switch {
	case a.Notify == nil && b.Notify == nil:
		return true
	case a.Notify == nil || b.Notify == nil:
		return false
	default:
		return *a.Notify == *b.Notify
	}
}
