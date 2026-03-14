package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"school-with-ai/infra/sqldb"
)

func main() {
	email := flag.String("email", "", "Email of the user to promote")
	role := flag.String("role", "admin", "Role to assign (default: admin)")
	dbHost := flag.String("db-host", "localhost:5435", "Database host:port")
	dbUser := flag.String("db-user", "appuser", "Database user")
	dbPass := flag.String("db-pass", "dev-password", "Database password")
	dbName := flag.String("db-name", "projections", "Database name")
	flag.Parse()

	if *email == "" {
		log.Fatal("Please provide an email using --email")
	}

	db, err := sqldb.Open(sqldb.Config{
		User:         *dbUser,
		Password:     *dbPass,
		Host:         *dbHost,
		Name:         *dbName,
		MaxIdleConns: 2,
		MaxOpenConns: 2,
		DisableTLS:   true,
	})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Printf("Searching for user with email: %s...\n", *email)

	var currentRole string
	var userID string
	err = db.QueryRowContext(ctx, "SELECT id, role FROM users WHERE email = $1", *email).Scan(&userID, &currentRole)
	if err != nil {
		log.Fatalf("User with email %s not found: %v", *email, err)
	}

	fmt.Printf("Found user: %s (ID: %s)\n", *email, userID)
	fmt.Printf("Current Role: %s\n", currentRole)

	_, err = db.ExecContext(ctx, "UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2", *role, userID)
	if err != nil {
		log.Fatalf("Error updating user role: %v", err)
	}

	fmt.Printf("Successfully promoted %s to %s\n", *email, *role)
}
