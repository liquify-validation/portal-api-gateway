package utils

import (
	"database/sql"

	_ "github.com/lib/pq"
)

func ConnectToDB(user, password, host, port, dbName string) (*sql.DB, error) {
	db, err := sql.Open("mysql", user+":"+password+"@tcp("+host+":"+port+")/"+dbName)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return db, nil
}

func QueryAPIKeyData(db *sql.DB, apiKey string) (map[string]interface{}, error) {
	var chain, org string
	var limit, orgID int
	stmt, err := db.Prepare("SELECT chain_name, org_name, `limit`, org_id FROM api_keys WHERE api_key = ?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	row := stmt.QueryRow(apiKey)
	err = row.Scan(&chain, &org, &limit, &orgID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"chain": chain,
		"org":   org,
		"orgID": orgID,
		"limit": limit,
	}, nil
}
