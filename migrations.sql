-- Each migration is delimited by a "-- migration: <name>" header.
-- Migrations are applied in order; applied names are tracked in
-- schema_migrations so each migration runs at most once.

-- migration: 001_add_contacts_notes
ALTER TABLE contacts ADD COLUMN notes TEXT NOT NULL DEFAULT '';
