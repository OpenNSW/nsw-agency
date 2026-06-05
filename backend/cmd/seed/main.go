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
	if len(os.Args) < 3 || os.Args[1] != "user" {
		usage()
		os.Exit(1)
	}

	switch os.Args[2] {
	case "add":
		runUserAdd(os.Args[3:])
	case "drop":
		runUserDrop()
	default:
		fmt.Fprintf(os.Stderr, "seed: unknown command %q\n\n", os.Args[2])
		usage()
		os.Exit(1)
	}
}

// ---------- user add ----------

type seedUser struct {
	SSOID string   `json:"ssoid"`
	Name  string   `json:"name"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

type seedFile struct {
	Users []seedUser `json:"users"`
}

// runUserAdd handles both file-based and interactive user seeding.
// If --file is provided, it reads from the JSON file; otherwise it prompts interactively.
func runUserAdd(args []string) {
	fs := flag.NewFlagSet("user add", flag.ExitOnError)
	fs.Usage = usage
	filePath := fs.String("file", "", "path to users JSON seed file")
	if err := fs.Parse(args); err != nil {
		fatalf("%v", err)
	}

	if *filePath != "" {
		runUserAddFromFile(*filePath)
	} else {
		runUserAddInteractive()
	}
}

func runUserAddFromFile(filePath string) {
	data, err := os.ReadFile(filePath)
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
	inserted, err := seedUsers(db, sf.Users)
	if err != nil {
		fatalf("%v", err)
	}
	fmt.Printf("seed: successfully seeded %d user(s)\n", inserted)
}

func runUserAddInteractive() {
	sc := bufio.NewScanner(os.Stdin)

	name := prompt(sc, "Name: ")
	if name == "" {
		fatalf("name is required")
	}
	email := prompt(sc, "Email: ")
	if email == "" {
		fatalf("email is required")
	}
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
	inserted, err := seedUsers(db, []seedUser{{Name: name, Email: email, Roles: roles}})
	if err != nil {
		fatalf("%v", err)
	}
	if inserted == 0 {
		fmt.Printf("seed: user %q already exists — skipped\n", email)
	} else {
		fmt.Printf("seed: user %q seeded successfully\n", email)
	}
}

// ---------- user drop ----------

func runUserDrop() {
	sc := bufio.NewScanner(os.Stdin)

	email := prompt(sc, "Email of user to drop: ")
	if email == "" {
		fatalf("email is required")
	}

	db := openGormDB()

	var u userRecord
	if err := db.First(&u, "email = ?", email).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fatalf("no user found with email %q", email)
		}
		fatalf("failed to find user: %v", err)
	}

	// ON DELETE CASCADE on user_roles.user_id removes role assignments automatically.
	if err := db.Delete(&u).Error; err != nil {
		fatalf("failed to drop user %q: %v", email, err)
	}

	fmt.Printf("seed: user %q dropped successfully\n", email)
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

func seedUsers(db *gorm.DB, users []seedUser) (int, error) {
	var inserted int
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		inserted, err = seedUsersInTx(tx, users)
		return err
	})
	return inserted, err
}

func seedUsersInTx(tx *gorm.DB, users []seedUser) (int, error) {
	roleStore := rbac.NewRoleStore(tx)
	userRoleStore := rbac.NewUserRoleStore(tx)

	// Deduplicate users by email — keep the first occurrence.
	seen := make(map[string]struct{})
	deduped := users[:0]
	for _, u := range users {
		if _, exists := seen[u.Email]; !exists {
			seen[u.Email] = struct{}{}
			deduped = append(deduped, u)
		}
	}
	users = deduped

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
			return 0, fmt.Errorf("upsert role %q: %w", name, err)
		}
		roleIndex[name] = role
	}

	// Upsert users and assign roles.
	inserted := 0
	for _, u := range users {
		var existing userRecord
		err := tx.First(&existing, "email = ?", u.Email).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			newUser := userRecord{Name: u.Name, Email: u.Email, SSOID: nullableSSID(u.SSOID)}
			if err := tx.Create(&newUser).Error; err != nil {
				return inserted, fmt.Errorf("create user %q: %w", u.Email, err)
			}
			existing = newUser
			inserted++
		} else if err != nil {
			return inserted, fmt.Errorf("fetch user %q: %w", u.Email, err)
		}

		for _, roleName := range u.Roles {
			role := roleIndex[roleName]
			if err := userRoleStore.Assign(existing.UserID, role.ID); err != nil {
				return inserted, fmt.Errorf("assign role %q to user %q: %w", roleName, u.Email, err)
			}
		}
	}
	return inserted, nil
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
	fmt.Fprint(os.Stderr, `Usage: seed user <command> [flags]

Commands:
  user add              Interactively add a single user and assign roles
  user add --file PATH  Seed users and roles from a JSON file
  user drop             Interactively remove a user by email (also removes their role assignments)

Flags for user add:
  --file <path>   Path to users JSON seed file (required for file-based seeding)

JSON file format:
  {
    "users": [
      {
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
