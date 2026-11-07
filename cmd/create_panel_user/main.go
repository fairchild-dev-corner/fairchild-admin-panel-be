package main

// create_panel_user adds a super admin or admin account to users_panel. There
// is no sign-up endpoint on purpose: this command creates the first super
// admin, who adds everyone else from the panel (System > Users, POST
// /admin/users), where managers, supervisors and staff get their capabilities.
//
//   make create-panel-user ENV=dev ARGS="-username jdelacruz -first-name Juan -last-name 'Dela Cruz' -role super_admin"
//
// The password is read from PANEL_USER_PASSWORD if set (for scripts),
// otherwise prompted for twice without echo.

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
	"time"

	conf "fairchild-admin-panel-be/config"
	cc "fairchild-admin-panel-be/internal/constants"
	models "fairchild-admin-panel-be/internal/models/auth"
	repository "fairchild-admin-panel-be/internal/repositories/auth"
	auth_service "fairchild-admin-panel-be/internal/services/auth"
	"fairchild-admin-panel-be/pkg/database"

	"github.com/google/uuid"
	"golang.org/x/term"
)

var BuildEnv string

func main() {
	username := flag.String("username", "", "login username (required)")
	firstName := flag.String("first-name", "", "first name (required)")
	lastName := flag.String("last-name", "", "last name (required)")
	email := flag.String("email", "", "email (optional)")
	role := flag.String("role", cc.ROLE_SUPER_ADMIN, "super_admin or admin")
	flag.Parse()

	u := strings.TrimSpace
	name := auth_service.NormalizeUsername(*username)
	if name == "" || u(*firstName) == "" || u(*lastName) == "" {
		flag.Usage()
		os.Exit(2)
	}
	if len(name) > 50 {
		log.Fatal("username must be at most 50 characters")
	}
	if *role != cc.ROLE_SUPER_ADMIN && *role != cc.ROLE_ADMIN {
		log.Fatalf("role must be %q or %q (add other roles from the panel)", cc.ROLE_SUPER_ADMIN, cc.ROLE_ADMIN)
	}

	password, err := readPassword()
	if err != nil {
		log.Fatal(err)
	}

	hash, err := auth_service.HashPassword(password)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	mysqlConf, err := conf.GetMySQLConfig(BuildEnv)
	if err != nil {
		log.Fatalf("MySQL config: %v", err)
	}
	db, err := database.MySQLInitConnector(mysqlConf, false)
	if err != nil {
		log.Fatalf("connect MySQL: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repo := repository.NewAuthRepository(db)
	id, err := repo.CreateUser(ctx, &models.UserPanel{
		UUID:         uuid.NewString(),
		Username:     name,
		PasswordHash: hash,
		FirstName:    u(*firstName),
		LastName:     u(*lastName),
		Email:        sql.NullString{String: u(*email), Valid: u(*email) != ""},
		Role:         *role,
		Status:       cc.USER_STATUS_ACTIVE,
	})
	if errors.Is(err, cc.ErrUsernameAlreadyExists) || errors.Is(err, cc.ErrEmailAlreadyExists) {
		log.Fatal(err)
	}
	if err != nil {
		log.Fatalf("create user: %v", err)
	}

	fmt.Printf("Created %s %q (users_panel.id=%d)\n", *role, name, id)
}

func readPassword() (string, error) {
	if p, ok := os.LookupEnv("PANEL_USER_PASSWORD"); ok {
		return p, checkPassword(p)
	}

	if !term.IsTerminal(int(syscall.Stdin)) {
		return "", errors.New("no terminal to prompt on - set PANEL_USER_PASSWORD instead")
	}

	fmt.Print("Password: ")
	first, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", err
	}
	fmt.Print("Confirm password: ")
	second, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", err
	}
	if string(first) != string(second) {
		return "", errors.New("passwords do not match")
	}
	return string(first), checkPassword(string(first))
}

func checkPassword(p string) error {
	if len(p) < auth_service.MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", auth_service.MinPasswordLength)
	}
	if len(p) > 128 {
		return errors.New("password must be at most 128 characters")
	}
	return nil
}
