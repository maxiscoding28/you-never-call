package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

func InitDB(path string) (*sql.DB, error) {
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}

	log.Println("Database initialized:", path)
	return db, nil
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE
		);

		CREATE TABLE IF NOT EXISTS contacts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			last_contact_date DATE,
			frequency_days INTEGER DEFAULT 30,
			group_id INTEGER,
			FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE SET NULL
		);
	`)
	return err
}

// --- Sort helpers ---

var allowedSortColumns = map[string]string{
	"first_name":        "c.first_name",
	"last_name":         "c.last_name",
	"last_contact_date": "c.last_contact_date",
	"group_name":        "g.name",
	"frequency_days":    "c.frequency_days",
}

func sanitizeSort(col, dir string) (string, string) {
	sqlCol, ok := allowedSortColumns[col]
	if !ok {
		sqlCol = "c.last_name"
	}
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}
	return sqlCol, dir
}

// --- Contact CRUD ---

func GetAllContacts(db *sql.DB, sortCol, sortDir string) ([]Contact, error) {
	col, dir := sanitizeSort(sortCol, sortDir)
	query := fmt.Sprintf(`
		SELECT c.id, c.first_name, c.last_name, c.last_contact_date,
		       c.frequency_days, c.group_id, COALESCE(g.name, '')
		FROM contacts c
		LEFT JOIN groups g ON c.group_id = g.id
		ORDER BY %s %s
	`, col, dir)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanContacts(rows)
}

func GetContactsByGroup(db *sql.DB, sortCol, sortDir string) (map[string][]Contact, error) {
	contacts, err := GetAllContacts(db, sortCol, sortDir)
	if err != nil {
		return nil, err
	}

	grouped := make(map[string][]Contact)
	for _, c := range contacts {
		key := c.GroupName
		if key == "" {
			key = "Ungrouped"
		}
		grouped[key] = append(grouped[key], c)
	}
	return grouped, nil
}

func GetContact(db *sql.DB, id int64) (Contact, error) {
	var c Contact
	var dateStr sql.NullString
	err := db.QueryRow(`
		SELECT c.id, c.first_name, c.last_name, c.last_contact_date,
		       c.frequency_days, c.group_id, COALESCE(g.name, '')
		FROM contacts c
		LEFT JOIN groups g ON c.group_id = g.id
		WHERE c.id = ?
	`, id).Scan(&c.ID, &c.FirstName, &c.LastName, &dateStr,
		&c.FrequencyDays, &c.GroupID, &c.GroupName)
	if err != nil {
		return c, err
	}
	if dateStr.Valid && dateStr.String != "" {
		t, err := parseDate(dateStr.String)
		if err == nil {
			c.LastContactDate = sql.NullTime{Time: t, Valid: true}
		}
	}
	c.Status = ComputeStatus(c.LastContactDate, c.FrequencyDays)
	return c, nil
}

func CreateContact(db *sql.DB, c Contact) error {
	var dateVal interface{}
	if c.LastContactDate.Valid {
		dateVal = c.LastContactDate.Time.Format("2006-01-02")
	}
	var groupVal interface{}
	if c.GroupID.Valid {
		groupVal = c.GroupID.Int64
	}
	_, err := db.Exec(`
		INSERT INTO contacts (first_name, last_name, last_contact_date, frequency_days, group_id)
		VALUES (?, ?, ?, ?, ?)
	`, c.FirstName, c.LastName, dateVal, c.FrequencyDays, groupVal)
	return err
}

func UpdateContact(db *sql.DB, c Contact) error {
	var dateVal interface{}
	if c.LastContactDate.Valid {
		dateVal = c.LastContactDate.Time.Format("2006-01-02")
	}
	var groupVal interface{}
	if c.GroupID.Valid {
		groupVal = c.GroupID.Int64
	}
	_, err := db.Exec(`
		UPDATE contacts
		SET first_name = ?, last_name = ?, last_contact_date = ?,
		    frequency_days = ?, group_id = ?
		WHERE id = ?
	`, c.FirstName, c.LastName, dateVal, c.FrequencyDays, groupVal, c.ID)
	return err
}

func DeleteContact(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM contacts WHERE id = ?", id)
	return err
}

// --- Group CRUD ---

func GetAllGroups(db *sql.DB) ([]Group, error) {
	rows, err := db.Query("SELECT id, name FROM groups ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func CreateGroup(db *sql.DB, name string) error {
	_, err := db.Exec("INSERT INTO groups (name) VALUES (?)", name)
	return err
}

func DeleteGroup(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM groups WHERE id = ?", id)
	return err
}

func UpdateGroup(db *sql.DB, g Group) error {
	_, err := db.Exec("UPDATE groups SET name = ? WHERE id = ?", g.Name, g.ID)
	return err
}

// --- Helpers ---

func scanContacts(rows *sql.Rows) ([]Contact, error) {
	var contacts []Contact
	for rows.Next() {
		var c Contact
		var dateStr sql.NullString
		if err := rows.Scan(&c.ID, &c.FirstName, &c.LastName, &dateStr,
			&c.FrequencyDays, &c.GroupID, &c.GroupName); err != nil {
			return nil, err
		}
		if dateStr.Valid && dateStr.String != "" {
			t, err := parseDate(dateStr.String)
			if err == nil {
				c.LastContactDate = sql.NullTime{Time: t, Valid: true}
			}
		}
		c.Status = ComputeStatus(c.LastContactDate, c.FrequencyDays)
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

func parseDate(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date: %s", s)
}
