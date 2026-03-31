# ☎ You Never Call

A local-first, minimalist CRM built with Go, SQLite, HTMX, and Pico.css.

Track your contacts, how often you should reach out, and who you're overdue to call.

## Prerequisites

- **Go 1.22+** — [Download](https://go.dev/dl/)

That's it. No Node.js, no Docker, no external database server.

## Quick Start

```bash
# Clone or navigate to the project directory
cd you-never-call

# Download dependencies
go mod tidy

# Run the server
go run .
```

Open **http://localhost:8080** in your browser.

The SQLite database file (`contacts.db`) is created automatically on first run. No manual setup required.

## Features

- **Contact Management** — Add, edit, and delete contacts with name, last contact date, and call frequency
- **Groups** — Organize contacts into groups (Family, Work, Friends, etc.)
- **Overdue Tracking** — Visual indicators show which contacts are overdue (red), due soon (orange), or OK (green)
- **Sortable Tables** — Click column headers to sort by name or date, ascending or descending — powered by HTMX, no page reload
- **Grouped View** — Toggle between a flat list and contacts organized by group
- **Clean Desk Design** — Minimal, high-contrast UI using Pico.css

## Tech Stack

| Layer    | Technology                         |
|----------|------------------------------------|
| Backend  | Go (standard library `net/http`)   |
| Database | SQLite via `modernc.org/sqlite`    |
| Frontend | HTML5, Pico.css, HTMX             |
| Routing  | Go 1.22+ `http.ServeMux` patterns  |

## Project Structure

```
you-never-call/
├── main.go              # Entry point, server startup
├── database.go          # SQLite init, CRUD operations
├── handlers.go          # HTTP handlers, template rendering
├── models.go            # Data types and status logic
├── go.mod
├── README.md
├── contacts.db          # Created at runtime
└── templates/
    ├── layout.html      # Base template (nav, CSS, JS)
    ├── index.html       # Dashboard with contact tables
    ├── contact_rows.html# HTMX partial for sorted rows
    ├── contact_edit.html# Contact create/edit form
    ├── groups.html      # Group management page
    └── group_list.html  # HTMX partial for group list
```

## How It Works

### Status Logic

Each contact has a `frequency_days` (default 30) and a `last_contact_date`. The status is computed in real time:

- **Overdue** — `last_contact_date + frequency_days < today` (or no date set)
- **Due Soon** — Within 3 days of the deadline
- **OK** — Still within the frequency window

### Database

SQLite with foreign keys enabled and WAL mode for performance. The schema is auto-created on startup:

- `groups` — id, name (unique)
- `contacts` — id, first_name, last_name, last_contact_date, frequency_days, group_id (FK → groups, ON DELETE SET NULL)

## License

MIT
