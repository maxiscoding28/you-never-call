package main

import (
	"database/sql"
	"time"
)

type Group struct {
	ID   int64
	Name string
}

type Contact struct {
	ID              int64
	FirstName       string
	LastName        string
	LastContactDate sql.NullTime
	FrequencyDays   int
	GroupID         sql.NullInt64
	GroupName       string
	Status          string
}

func ComputeStatus(lastContactDate sql.NullTime, frequencyDays int) string {
	if !lastContactDate.Valid {
		return "Overdue"
	}
	deadline := lastContactDate.Time.AddDate(0, 0, frequencyDays)
	now := time.Now()
	if now.After(deadline) {
		return "Overdue"
	}
	if now.After(deadline.AddDate(0, 0, -3)) {
		return "Due Soon"
	}
	return "OK"
}

func DaysUntilDue(lastContactDate sql.NullTime, frequencyDays int) int {
	if !lastContactDate.Valid {
		return -999
	}
	deadline := lastContactDate.Time.AddDate(0, 0, frequencyDays)
	return int(time.Until(deadline).Hours() / 24)
}
