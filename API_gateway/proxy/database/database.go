package database

import (
	"time"

	"database/sql"

	"strconv"

	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"proxy/config"
)

var db *sql.DB

func InitDB() (*sql.DB, error) {
	dbUser, dbPassword, dbHost, dbPort, dbDatabaseName := config.LoadDBConfig()
	dsn := dbUser + ":" + dbPassword + "@tcp(" + dbHost + ":" + dbPort + ")/" + dbDatabaseName + "?parseTime=true"

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// Optional: tune pool settings
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)

	// Test connection
	if err = db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func InitTimescaleDB() (*sql.DB, error) {
    tsUser, tsPassword, tsHost, tsPort, tsDatabaseName := config.LoadTimescaleConfig()

    dsn := fmt.Sprintf(
        "host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
        tsHost, tsPort, tsUser, tsPassword, tsDatabaseName,
    )

    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, err
    }

    db.SetMaxOpenConns(50)
    db.SetMaxIdleConns(10)
    db.SetConnMaxLifetime(time.Hour)

    if err = db.Ping(); err != nil {
        return nil, err
    }

    return db, nil
}

func FetchAPIKeyInfo(db *sql.DB, apiKey string) (map[string]interface{}, error) {
	query := "SELECT chain_name, org_name, `limit`, org_id FROM api_keys WHERE api_key = ?"
	row := db.QueryRow(query, apiKey)

	var chain, org string
	var limit, orgID int
	err := row.Scan(&chain, &org, &limit, &orgID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"chain": chain, "org": org, "limit": limit, "org_id": strconv.Itoa(orgID),
	}, nil
}