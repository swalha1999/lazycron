package envsubst

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Substitute ---

func TestSubstitute_Basic(t *testing.T) {
	vars := map[string]string{
		"DB_HOST": "localhost",
		"DB_NAME": "mydb",
	}
	input := "pg_dump -h ${DB_HOST} ${DB_NAME}"
	got, err := Substitute(input, vars)
	if err != nil {
		t.Fatalf("Substitute: %v", err)
	}
	want := "pg_dump -h localhost mydb"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSubstitute_NoVars(t *testing.T) {
	input := "echo hello world"
	got, err := Substitute(input, map[string]string{})
	if err != nil {
		t.Fatalf("Substitute: %v", err)
	}
	if got != input {
		t.Errorf("got %q, want %q", got, input)
	}
}

func TestSubstitute_UndefinedVar(t *testing.T) {
	vars := map[string]string{"DB_HOST": "localhost"}
	input := "pg_dump -h ${DB_HOST} ${DB_NAME}"
	_, err := Substitute(input, vars)
	if err == nil {
		t.Fatal("expected error for undefined variable")
	}
	if !strings.Contains(err.Error(), "DB_NAME") {
		t.Errorf("error should mention DB_NAME: %v", err)
	}
}

func TestSubstitute_MultipleUndefined(t *testing.T) {
	input := "${A} ${B} ${C}"
	_, err := Substitute(input, map[string]string{})
	if err == nil {
		t.Fatal("expected error for undefined variables")
	}
	for _, name := range []string{"A", "B", "C"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error should mention %s: %v", name, err)
		}
	}
}

func TestSubstitute_RepeatedVar(t *testing.T) {
	vars := map[string]string{"X": "42"}
	input := "${X} and ${X}"
	got, err := Substitute(input, vars)
	if err != nil {
		t.Fatalf("Substitute: %v", err)
	}
	if got != "42 and 42" {
		t.Errorf("got %q, want %q", got, "42 and 42")
	}
}

func TestSubstitute_PreservesNonVarDollar(t *testing.T) {
	vars := map[string]string{}
	// $FOO (no braces) should not be substituted.
	input := "date +%F $HOME"
	got, err := Substitute(input, vars)
	if err != nil {
		t.Fatalf("Substitute: %v", err)
	}
	if got != input {
		t.Errorf("got %q, want %q", got, input)
	}
}

// --- ParseEnvFile ---

func TestParseEnvFile_Basic(t *testing.T) {
	content := "DB_HOST=localhost\nDB_NAME=mydb\nBACKUP_DIR=/backups\n"
	path := writeEnvFile(t, content)

	vars, err := ParseEnvFile(path)
	if err != nil {
		t.Fatalf("ParseEnvFile: %v", err)
	}

	want := map[string]string{
		"DB_HOST":    "localhost",
		"DB_NAME":    "mydb",
		"BACKUP_DIR": "/backups",
	}
	for k, v := range want {
		if vars[k] != v {
			t.Errorf("vars[%q] = %q, want %q", k, vars[k], v)
		}
	}
}

func TestParseEnvFile_CommentsAndBlankLines(t *testing.T) {
	content := "# This is a comment\n\nDB_HOST=localhost\n  # Another comment\n\nDB_NAME=mydb\n"
	path := writeEnvFile(t, content)

	vars, err := ParseEnvFile(path)
	if err != nil {
		t.Fatalf("ParseEnvFile: %v", err)
	}
	if len(vars) != 2 {
		t.Errorf("expected 2 vars, got %d", len(vars))
	}
}

func TestParseEnvFile_QuotedValues(t *testing.T) {
	content := `DB_HOST="localhost"
DB_NAME='mydb'
PLAIN=value
`
	path := writeEnvFile(t, content)

	vars, err := ParseEnvFile(path)
	if err != nil {
		t.Fatalf("ParseEnvFile: %v", err)
	}

	if vars["DB_HOST"] != "localhost" {
		t.Errorf("DB_HOST = %q, want %q", vars["DB_HOST"], "localhost")
	}
	if vars["DB_NAME"] != "mydb" {
		t.Errorf("DB_NAME = %q, want %q", vars["DB_NAME"], "mydb")
	}
	if vars["PLAIN"] != "value" {
		t.Errorf("PLAIN = %q, want %q", vars["PLAIN"], "value")
	}
}

func TestParseEnvFile_ValueWithEquals(t *testing.T) {
	content := "CONNECTION=host=localhost dbname=mydb\n"
	path := writeEnvFile(t, content)

	vars, err := ParseEnvFile(path)
	if err != nil {
		t.Fatalf("ParseEnvFile: %v", err)
	}
	if vars["CONNECTION"] != "host=localhost dbname=mydb" {
		t.Errorf("CONNECTION = %q", vars["CONNECTION"])
	}
}

func TestParseEnvFile_InvalidLine(t *testing.T) {
	content := "not-a-valid-line\n"
	path := writeEnvFile(t, content)

	_, err := ParseEnvFile(path)
	if err == nil {
		t.Fatal("expected error for invalid line")
	}
}

func TestParseEnvFile_NotFound(t *testing.T) {
	_, err := ParseEnvFile("/nonexistent/.env")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// --- BuildVarMap ---

func TestBuildVarMap_FlagOverridesEnvFile(t *testing.T) {
	envContent := "DB_HOST=from-env-file\n"
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte(envContent), 0o644); err != nil {
		t.Fatal(err)
	}

	vars, err := BuildVarMap([]string{"DB_HOST=from-flag"}, envPath)
	if err != nil {
		t.Fatalf("BuildVarMap: %v", err)
	}
	if vars["DB_HOST"] != "from-flag" {
		t.Errorf("DB_HOST = %q, want %q", vars["DB_HOST"], "from-flag")
	}
}

func TestBuildVarMap_EnvFileOverridesShell(t *testing.T) {
	t.Setenv("TEST_LAZYCRON_VAR", "from-shell")

	envContent := "TEST_LAZYCRON_VAR=from-env-file\n"
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte(envContent), 0o644); err != nil {
		t.Fatal(err)
	}

	vars, err := BuildVarMap(nil, envPath)
	if err != nil {
		t.Fatalf("BuildVarMap: %v", err)
	}
	if vars["TEST_LAZYCRON_VAR"] != "from-env-file" {
		t.Errorf("TEST_LAZYCRON_VAR = %q, want %q", vars["TEST_LAZYCRON_VAR"], "from-env-file")
	}
}

func TestBuildVarMap_ShellEnvFallback(t *testing.T) {
	t.Setenv("TEST_LAZYCRON_SHELL", "from-shell")

	vars, err := BuildVarMap(nil, "")
	if err != nil {
		t.Fatalf("BuildVarMap: %v", err)
	}
	if vars["TEST_LAZYCRON_SHELL"] != "from-shell" {
		t.Errorf("TEST_LAZYCRON_SHELL = %q, want %q", vars["TEST_LAZYCRON_SHELL"], "from-shell")
	}
}

func TestBuildVarMap_MissingEnvFileIgnored(t *testing.T) {
	vars, err := BuildVarMap(nil, "/nonexistent/.env")
	if err != nil {
		t.Fatalf("BuildVarMap: %v", err)
	}
	if vars == nil {
		t.Fatal("expected non-nil map")
	}
}

func TestBuildVarMap_InvalidFlagFormat(t *testing.T) {
	_, err := BuildVarMap([]string{"NOEQUALS"}, "")
	if err == nil {
		t.Fatal("expected error for invalid --var format")
	}
}

// --- helpers ---

func writeEnvFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
