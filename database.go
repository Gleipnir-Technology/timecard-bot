package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"sync"

	//"github.com/georgysavva/scany/v2/pgxscan"
	//"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stephenafamo/bob"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type postgres struct {
	BobDB bob.DB
}

var (
	PGInstance *postgres
	pgOnce     sync.Once
)

func doMigrations(connection_string string) error {
	log.Println("Connecting to database at", connection_string)
	db, err := sql.Open("pgx", connection_string)
	if err != nil {
		return fmt.Errorf("Failed to open database connection: %w", err)
	}
	defer db.Close()
	row := db.QueryRowContext(context.Background(), "SELECT version()")
	var val string
	if err := row.Scan(&val); err != nil {
		return fmt.Errorf("Failed to get database version query result: %w", err)
	}
	log.Printf("Connected to: %s", val)

	fsys, err := fs.Sub(embedMigrations, "migrations")
	if err != nil {
		return fmt.Errorf("Failed to get migrations embedded directory: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys)
	if err != nil {
		return fmt.Errorf("Failed to create goose provider: %w", err)
	}
	//goose.SetBaseFS(embedMigrations)

	current, target, err := provider.GetVersions(context.Background())
	if err != nil {
		return fmt.Errorf("Faield to get goose versions: %w", err)
	}
	log.Printf("Current version %d, need to be at version %d", current, target)
	results, err := provider.Up(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to run migrations: %w", err)
	}
	if len(results) > 0 {
		for _, r := range results {
			log.Printf("Migration %d %s", r.Source.Version, r.Direction)
		}
	} else {
		log.Println("No migrations necessary.")
	}
	return nil
}

func initializeDatabase(ctx context.Context, uri string) error {
	needs, err := needsMigrations(uri)
	if err != nil {
		return fmt.Errorf("Failed to determine if migrations are needed: %v", err)
	}
	if needs == nil {
		return errors.New("Can't read variable 'needs' - it's nil")
	}
	if *needs {
		return errors.New(fmt.Sprintf("Must migrate database before connecting: %t", *needs))
	}

	pgOnce.Do(func() {
		db, e := pgxpool.New(ctx, uri)
		bobDB := bob.NewDB(stdlib.OpenDBFromPool(db))
		PGInstance = &postgres{bobDB}
		err = e
	})
	if err != nil {
		return fmt.Errorf("unable to create connection pool: %w", err)
	}

	var current string
	query := `SELECT current_database()`
	err = PGInstance.BobDB.QueryRow(query).Scan(&current)
	if err != nil {
		return fmt.Errorf("Failed to get database current: %w", err)
	}
	log.Println("Connected to", current)
	return nil
}

func needsMigrations(connection_string string) (*bool, error) {
	log.Println("Connecting to database at", connection_string)
	db, err := sql.Open("pgx", connection_string)
	if err != nil {
		return nil, fmt.Errorf("Failed to open database connection: %w", err)
	}
	defer db.Close()
	row := db.QueryRowContext(context.Background(), "SELECT version()")
	var val string
	if err := row.Scan(&val); err != nil {
		return nil, fmt.Errorf("Failed to get database version query result: %w", err)
	}
	log.Printf("Connected to: %s", val)

	fsys, err := fs.Sub(embedMigrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("Failed to get migrations embedded directory: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys)
	if err != nil {
		return nil, fmt.Errorf("Failed to create goose provider: %w", err)
	}

	hasPending, err := provider.HasPending(context.Background())
	if err != nil {
		return nil, err
	}
	return &hasPending, nil
}
