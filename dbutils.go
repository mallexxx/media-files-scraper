package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

func openDB(dbPath string) (*sql.DB, error) {
	// Open or create the SQLite database
	return sql.Open("sqlite3", dbPath)
}

func initializeDB(dbPath string) (*sql.DB, error) {
	db, err := openDB(dbPath)
	if err != nil {
		return nil, err
	}

	// Ensure the necessary tables are created
	err = createSQLiteTables(db)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func createSQLiteTables(db *sql.DB) error {
	// Create mediaEntities table
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS mediaEntities (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT NOT NULL COLLATE NOCASE UNIQUE,
		CONSTRAINT idx_path UNIQUE (path)
	);`)
	if err != nil {
		return err
	}

	// Create files table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		entityId INTEGER,
		relativePath TEXT COLLATE NOCASE,
		linkPath TEXT COLLATE NOCASE NOT NULL,
		FOREIGN KEY (entityId) REFERENCES mediaEntities(id)
	);`)
	if err != nil {
		return err
	}

	return nil
}

func insertMediaEntity(db *sql.DB, filePath string) (int64, error) {
	var lastInsertID int64
	err := db.QueryRow("INSERT OR IGNORE INTO mediaEntities (path) VALUES (?) RETURNING id", filePath).Scan(&lastInsertID)
	if err == nil {
		Logf("added %s", filePath)
	} else {
		if err == sql.ErrNoRows {
			// No rows were inserted, perform the SELECT query to get the existing ID
			err = db.QueryRow("SELECT id FROM mediaEntities WHERE path = ?", filePath).Scan(&lastInsertID)
			if err != nil {
				return 0, err
			}
		} else {
			return 0, err
		}
	}

	return lastInsertID, nil
}

func deleteMissingEntities(db *sql.DB, insertedIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Create a temporary table to hold inserted IDs
	_, err = tx.Exec("CREATE TEMPORARY TABLE temp_inserted_ids (id INTEGER PRIMARY KEY)")
	if err != nil {
		return err
	}

	// Insert insertedIDs into the temporary table
	stmt, err := tx.Prepare("INSERT INTO temp_inserted_ids (id) VALUES (?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, id := range insertedIDs {
		_, err = stmt.Exec(id)
		if err != nil {
			return err
		}
	}

	// Delete missing entities and related files
	_, err = tx.Exec("DELETE FROM mediaEntities WHERE id NOT IN (SELECT id FROM temp_inserted_ids)")
	if err != nil {
		return err
	}
	_, err = tx.Exec("DELETE FROM files WHERE entityId NOT IN (SELECT id FROM temp_inserted_ids)")
	if err != nil {
		return err
	}

	return tx.Commit()
}
