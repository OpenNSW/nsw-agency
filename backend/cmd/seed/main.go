package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/OpenNSW/nsw-agency/backend/internal/database"
	"github.com/OpenNSW/nsw-agency/backend/internal/rbac"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "add-users":
		runAddUsers(os.Args[2:])
	case "add-user":
		runAddUser()
	default:
		fmt.Fprintf(os.Stderr, "seed: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

// ---------- add-users (file-based) ----------

type seedUser struct {
	SSOID string   `json:"ssoid"`
	Name  string   `json:"name"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

type seedFile struct {
	Users []seedUser `json:"users"`
}

func runAddUsers(args []string) {
	fs := flag.NewFlagSet("add-users", flag.ExitOnError)
	filePath := fs.String("file", "data/seed/users.json", "path to users JSON seed file")
	if err := fs.Parse(args); err != nil {
		fatalf("%v", err)
	}

	data, err := os.ReadFile(*filePath)
	if err != nil {
		fatalf("read file: %v", err)
	}

	var sf seedFile
	if err := json.Unmarshal(data, &sf); err != nil {
		fatalf("parse JSON: %v", err)
	}
	if len(sf.Users) == 0 {
		fmt.Println("seed: no users found in file, nothing to do")
		return
	}

	db := openGormDB()
	if err := seedUsers(db, sf.Users); err != nil {
		fatalf("%v", err)
	}
	fmt.Printf("seed: successfully seeded %d user(s)\n", len(sf.Users))
}

// ---------- add-user (interactive) ----------

func runAddUser() {
	sc := bufio.NewScanner(os.Stdin)

	name := prompt(sc, "Name: ")
	if name == "" {
		fatalf("name is required")
	}
	email := prompt(sc, "Email: ")
	if email == "" {
		fatalf("email is required")
	}
	ssoid := prompt(sc, "SSOID: ")
	rolesInput := prompt(sc, "Roles (comma-separated): ")

	var roles []string
	for _, r := range strings.Split(rolesInput, ",") {
		if trimmed := strings.TrimSpace(r); trimmed != "" {
			roles = append(roles, trimmed)
		}
	}
	if len(roles) == 0 {
		fatalf("at least one role is required")
	}

	db := openGormDB()
	if err := seedUsers(db, []seedUser{{SSOID: ssoid, Name: name, Email: email, Roles: roles}}); err != nil {
		fatalf("%v", err)
	}
	fmt.Printf("seed: user %q seeded successfully\n", email)
}

// ---------- seeding logic ----------

// userRecord mirrors the users table. Defined here to avoid importing the
// user store's agency-validation logic which is not appropriate for seeding.
type userRecord struct {
	UserID    string    `gorm:"type:text;primaryKey;column:user_id"`
	SSOID     *string   `gorm:"column:ssoid;type:text;uniqueIndex"`
	Email     string    `gorm:"type:text"`
	Name      string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (userRecord) TableName() string { return "users" }

func (u *userRecord) BeforeCreate(_ *gorm.DB) error {
	if u.UserID == "" {
		u.UserID = uuid.New().String()
	}
	return nil
}

func seedUsers(db *gorm.DB, users []seedUser) error {
	return db.Transaction(func(tx *gorm.DB) error {
		return seedUsersInTx(tx, users)
	})
}

func seedUsersInTx(tx *gorm.DB, users []seedUser) error {
	roleStore := rbac.NewRoleStore(tx)
	userRoleStore := rbac.NewUserRoleStore(tx)

	// Collect unique role names across all users.
	roleNames := make(map[string]struct{})
	for _, u := range users {
		for _, r := range u.Roles {
			roleNames[r] = struct{}{}
		}
	}

	// Upsert roles — create if not exists, reuse existing.
	roleIndex := make(map[string]*rbac.RoleRecord)
	for name := range roleNames {
		role, err := roleStore.FindByName(name)
		if errors.Is(err, rbac.ErrRoleNotFound) {
			role, err = roleStore.Create(name)
		}
		if err != nil {
			return fmt.Errorf("upsert role %q: %w", name, err)
		}
		roleIndex[name] = role
	}

	// Upsert users and assign roles.
	for _, u := range users {
		var existing userRecord
		err := tx.First(&existing, "email = ?", u.Email).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			newUser := userRecord{Name: u.Name, Email: u.Email, SSOID: nullableSSID(u.SSOID)}
			if err := tx.Create(&newUser).Error; err != nil {
				return fmt.Errorf("create user %q: %w", u.Email, err)
			}
			existing = newUser
		} else if err != nil {
			return fmt.Errorf("fetch user %q: %w", u.Email, err)
		}

		for _, roleName := range u.Roles {
			role := roleIndex[roleName]
			if err := userRoleStore.Assign(existing.UserID, role.ID); err != nil {
				// Ignore duplicate assignments.
				if !strings.Contains(err.Error(), "UNIQUE constraint failed") &&
					!strings.Contains(err.Error(), "duplicate key") {
					return fmt.Errorf("assign role %q to user %q: %w", roleName, u.Email, err)
				}
			}
		}
	}
	return nil
}

// ---------- helpers ----------

func openGormDB() *gorm.DB {
	cfg, err := LoadConfig()
	if err != nil {
		fatalf("config: %v", err)
	}
	db, err := openDB(cfg.DB)
	if err != nil {
		fatalf("open database: %v", err)
	}
	return db
}

func openDB(cfg database.Config) (*gorm.DB, error) {
	connector, err := database.NewConnector(cfg)
	if err != nil {
		return nil, err
	}
	return connector.Open()
}

func prompt(sc *bufio.Scanner, label string) string {
	fmt.Print(label)
	sc.Scan()
	return strings.TrimSpace(sc.Text())
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage: seed <command> [flags]

Commands:
  add-users   Seed users and roles from a JSON file
  add-user    Interactively add a single user and assign roles

Flags for add-users:
  --file <path>   Path to users JSON seed file (default: data/seed/users.json)

JSON file format:
  {
    "users": [
      {
        "ssoid": "abc-123",
        "name": "Jane Doe",
        "email": "jane@agency.gov.au",
        "roles": ["lab_officer", "lab_manager"]
      }
    ]
  }

Environment variables:
  DB_DRIVER     sqlite or postgres (default: sqlite)
  DB_PATH       SQLite file path (default: ./agency_applications.db)
  DB_HOST       PostgreSQL host (default: localhost)
  DB_PORT       PostgreSQL port (default: 5432)
  DB_USER       PostgreSQL user (default: postgres)
  DB_PASSWORD   PostgreSQL password (required for postgres)
  DB_NAME       PostgreSQL database name (default: nsw_agency_db)
  DB_SSLMODE    PostgreSQL SSL mode (default: disable)
`)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "seed: "+format+"\n", args...)
	os.Exit(1)
}

// nullableSSID returns nil when ssoid is empty so it is stored as NULL in the
// database, allowing multiple unseeded users without violating the UNIQUE constraint.
func nullableSSID(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
