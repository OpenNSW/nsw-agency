package migrator

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Migration holds the parsed SQL for a single migration file.
type Migration struct {
	Version int64
	Name    string
	Up      string
	Down    string
}

// ParseFile reads a .sql migration file and extracts the -- @UP and -- @DOWN blocks.
// File names must follow the convention: <version>_<name>.sql, e.g. 000001_create_users.sql.
func ParseFile(path string) (*Migration, error) {
	base := filepath.Base(path)
	version, name, err := parseFilename(base)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	up, down, err := parseBlocks(string(content))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", base, err)
	}

	return &Migration{
		Version: version,
		Name:    name,
		Up:      up,
		Down:    down,
	}, nil
}

func parseFilename(base string) (int64, string, error) {
	name := strings.TrimSuffix(base, ".sql")
	idx := strings.Index(name, "_")
	if idx < 0 {
		return 0, "", fmt.Errorf("migration filename %q must follow the pattern <version>_<name>.sql", base)
	}
	version, err := strconv.ParseInt(name[:idx], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("migration filename %q: version prefix %q is not a number", base, name[:idx])
	}
	return version, name[idx+1:], nil
}

// parseBlocks splits file content into UP and DOWN SQL sections
// delimited by the -- @UP and -- @DOWN annotations.
// Matching is case-insensitive and space-insensitive (e.g. "--@up", "-- @UP", "--  @Down" all match).
func parseBlocks(content string) (up, down string, err error) {
	var (
		section   string
		upLines   []string
		downLines []string
	)

	for _, line := range strings.Split(content, "\n") {
		normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(line), " ", ""))
		switch normalized {
		case "--@UP":
			section = "up"
		case "--@DOWN":
			section = "down"
		default:
			switch section {
			case "up":
				upLines = append(upLines, line)
			case "down":
				downLines = append(downLines, line)
			}
		}
	}

	up = strings.TrimSpace(strings.Join(upLines, "\n"))
	if up == "" {
		return "", "", fmt.Errorf("missing -- @UP annotation")
	}
	down = strings.TrimSpace(strings.Join(downLines, "\n"))
	return up, down, nil
}
