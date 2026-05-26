package migrator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFilename(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantVersion int64
		wantName    string
		wantErr     bool
	}{
		{"valid", "000001_create_users.sql", 1, "create_users", false},
		{"multi-underscore", "000002_create_application_table.sql", 2, "create_application_table", false},
		{"no underscore", "create_users.sql", 0, "", true},
		{"non-numeric version", "abc_create_users.sql", 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, name, err := parseFilename(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseFilename(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr {
				if version != tt.wantVersion {
					t.Errorf("version = %d, want %d", version, tt.wantVersion)
				}
				if name != tt.wantName {
					t.Errorf("name = %q, want %q", name, tt.wantName)
				}
			}
		})
	}
}

func TestParseBlocks(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantUp   string
		wantDown string
		wantErr  bool
	}{
		{
			name: "up and down",
			content: `-- @UP
CREATE TABLE users (id INTEGER PRIMARY KEY);

-- @DOWN
DROP TABLE users;`,
			wantUp:   "CREATE TABLE users (id INTEGER PRIMARY KEY);",
			wantDown: "DROP TABLE users;",
		},
		{
			name: "up only",
			content: `-- @UP
CREATE TABLE users (id INTEGER PRIMARY KEY);`,
			wantUp:   "CREATE TABLE users (id INTEGER PRIMARY KEY);",
			wantDown: "",
		},
		{
			name:    "missing up",
			content: `-- @DOWN\nDROP TABLE users;`,
			wantErr: true,
		},
		{
			name:     "no space variant --@UP",
			content:  "--@UP\nCREATE TABLE users (id INTEGER PRIMARY KEY);\n--@DOWN\nDROP TABLE users;",
			wantUp:   "CREATE TABLE users (id INTEGER PRIMARY KEY);",
			wantDown: "DROP TABLE users;",
		},
		{
			name:     "lowercase variant",
			content:  "-- @up\nCREATE TABLE users (id INTEGER PRIMARY KEY);\n-- @down\nDROP TABLE users;",
			wantUp:   "CREATE TABLE users (id INTEGER PRIMARY KEY);",
			wantDown: "DROP TABLE users;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			up, down, err := parseBlocks(tt.content)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseBlocks() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if up != tt.wantUp {
					t.Errorf("up = %q, want %q", up, tt.wantUp)
				}
				if down != tt.wantDown {
					t.Errorf("down = %q, want %q", down, tt.wantDown)
				}
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "000001_create_users.sql")
	content := "-- @UP\nCREATE TABLE users (id INTEGER PRIMARY KEY);\n\n-- @DOWN\nDROP TABLE users;\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	mg, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if mg.Version != 1 {
		t.Errorf("Version = %d, want 1", mg.Version)
	}
	if mg.Name != "create_users" {
		t.Errorf("Name = %q, want create_users", mg.Name)
	}
	if mg.Up == "" {
		t.Error("Up block is empty")
	}
	if mg.Down == "" {
		t.Error("Down block is empty")
	}
}
