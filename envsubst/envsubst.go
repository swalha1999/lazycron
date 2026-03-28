package envsubst

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// varPattern matches ${VAR_NAME} references in text.
var varPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// Substitute replaces all ${VAR} references in content using the provided
// variable map. Variables not found in the map are left unchanged (allowing
// bash variables and other runtime variables to pass through).
func Substitute(content string, vars map[string]string) (string, error) {
	result := varPattern.ReplaceAllStringFunc(content, func(match string) string {
		name := varPattern.FindStringSubmatch(match)[1]
		if val, ok := vars[name]; ok {
			return val
		}
		// Leave undefined variables unchanged (e.g., bash variables)
		return match
	})

	return result, nil
}

// ParseEnvFile reads a .env file and returns key=value pairs.
// Empty lines and lines starting with # are ignored.
func ParseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: invalid line (expected KEY=VALUE)", path, lineNum)
		}

		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		// Strip optional surrounding quotes.
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}

		vars[key] = val
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	return vars, nil
}

// BuildVarMap builds a variable map by merging sources in priority order:
//  1. flagVars (--var KEY=VALUE flags, highest priority)
//  2. envFile (.lazycron/.env file, if it exists)
//  3. os.Environ (shell environment, lowest priority)
func BuildVarMap(flagVars []string, envFilePath string) (map[string]string, error) {
	vars := make(map[string]string)

	// Layer 3: shell environment (lowest priority).
	for _, env := range os.Environ() {
		if key, val, ok := strings.Cut(env, "="); ok {
			vars[key] = val
		}
	}

	// Layer 2: .env file (overrides shell env).
	if envFilePath != "" {
		if _, err := os.Stat(envFilePath); err == nil {
			envVars, err := ParseEnvFile(envFilePath)
			if err != nil {
				return nil, err
			}
			for k, v := range envVars {
				vars[k] = v
			}
		}
	}

	// Layer 1: --var flags (highest priority).
	for _, fv := range flagVars {
		key, val, ok := strings.Cut(fv, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --var format %q (expected KEY=VALUE)", fv)
		}
		vars[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}

	return vars, nil
}
