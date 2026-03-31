package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Handlers struct {
	db    *sql.DB
	pages map[string]*template.Template
}

func NewHandlers(db *sql.DB) *Handlers {
	funcMap := template.FuncMap{
		"formatDate": func(t sql.NullTime) string {
			if !t.Valid {
				return "—"
			}
			return t.Time.Format("Jan 2, 2006")
		},
		"formatDateInput": func(t sql.NullTime) string {
			if !t.Valid {
				return ""
			}
			return t.Time.Format("2006-01-02")
		},
		"sortArrow": func(currentSort, currentDir, col string) template.HTML {
			if currentSort == col {
				if currentDir == "asc" {
					return " ▲"
				}
				return " ▼"
			}
			return ""
		},
		"nextDir": func(currentSort, currentDir, col string) string {
			if currentSort == col && currentDir == "asc" {
				return "desc"
			}
			return "asc"
		},
		"groupIDValue": func(gid sql.NullInt64) int64 {
			if gid.Valid {
				return gid.Int64
			}
			return 0
		},
		"daysUntil": DaysUntilDue,
	}

	pages := make(map[string]*template.Template)
	// Each page gets its own template set so {{define "content"}} blocks don't collide
	pages["index"] = template.Must(
		template.New("").Funcs(funcMap).ParseFiles("templates/layout.html", "templates/index.html"),
	)
	pages["contact_edit"] = template.Must(
		template.New("").Funcs(funcMap).ParseFiles("templates/layout.html", "templates/contact_edit.html"),
	)
	pages["groups"] = template.Must(
		template.New("").Funcs(funcMap).ParseFiles("templates/layout.html", "templates/groups.html", "templates/group_list.html"),
	)
	// Partials for HTMX responses (no layout wrapper needed)
	pages["partials"] = template.Must(
		template.New("").Funcs(funcMap).ParseFiles("templates/index.html", "templates/group_list.html"),
	)

	return &Handlers{db: db, pages: pages}
}

func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.handleIndex)
	mux.HandleFunc("GET /contacts/rows", h.handleContactRows)
	mux.HandleFunc("GET /contact/new", h.handleContactNew)
	mux.HandleFunc("POST /contact/new", h.handleContactCreate)
	mux.HandleFunc("GET /contact/{id}", h.handleContactEdit)
	mux.HandleFunc("POST /contact/{id}/update", h.handleContactUpdate)
	mux.HandleFunc("POST /contact/{id}/delete", h.handleContactDelete)
	mux.HandleFunc("GET /groups", h.handleGroups)
	mux.HandleFunc("POST /groups/create", h.handleGroupCreate)
	mux.HandleFunc("POST /groups/{id}/delete", h.handleGroupDelete)
}

// --- Dashboard ---

func (h *Handlers) handleIndex(w http.ResponseWriter, r *http.Request) {
	sortCol := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")
	view := r.URL.Query().Get("view")

	if sortCol == "" {
		sortCol = "last_name"
	}
	if sortDir == "" {
		sortDir = "asc"
	}
	if view == "" {
		view = "all"
	}

	data := map[string]interface{}{
		"Sort": sortCol,
		"Dir":  sortDir,
		"View": view,
	}

	if view == "grouped" {
		grouped, err := GetContactsByGroup(h.db, sortCol, sortDir)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		groupNames := sortedGroupNames(grouped)
		data["GroupedContacts"] = grouped
		data["GroupNames"] = groupNames
	} else {
		contacts, err := GetAllContacts(h.db, sortCol, sortDir)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		data["Contacts"] = contacts
	}

	h.renderPage(w, "index", "index.html", data)
}

// --- HTMX partial: sorted contact rows ---

func (h *Handlers) handleContactRows(w http.ResponseWriter, r *http.Request) {
	sortCol := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")
	view := r.URL.Query().Get("view")

	if sortCol == "" {
		sortCol = "last_name"
	}
	if sortDir == "" {
		sortDir = "asc"
	}

	data := map[string]interface{}{
		"Sort": sortCol,
		"Dir":  sortDir,
		"View": view,
	}

	if view == "grouped" {
		grouped, err := GetContactsByGroup(h.db, sortCol, sortDir)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		groupNames := sortedGroupNames(grouped)
		data["GroupedContacts"] = grouped
		data["GroupNames"] = groupNames
		h.renderPartial(w, "grouped_tables", data)
	} else {
		contacts, err := GetAllContacts(h.db, sortCol, sortDir)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		data["Contacts"] = contacts
		h.renderPartial(w, "all_table", data)
	}
}

// --- Contact CRUD handlers ---

func (h *Handlers) handleContactNew(w http.ResponseWriter, r *http.Request) {
	groups, err := GetAllGroups(h.db)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	data := map[string]interface{}{
		"Contact": Contact{FrequencyDays: 30},
		"Groups":  groups,
		"IsNew":   true,
	}
	h.renderPage(w, "contact_edit", "contact_edit.html", data)
}

func (h *Handlers) handleContactCreate(w http.ResponseWriter, r *http.Request) {
	c, err := parseContactForm(r)
	if err != nil {
		http.Error(w, "Invalid form data: "+err.Error(), 400)
		return
	}
	if err := CreateContact(h.db, c); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) handleContactEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}
	contact, err := GetContact(h.db, id)
	if err != nil {
		http.Error(w, "Contact not found", 404)
		return
	}
	groups, err := GetAllGroups(h.db)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	data := map[string]interface{}{
		"Contact": contact,
		"Groups":  groups,
		"IsNew":   false,
	}
	h.renderPage(w, "contact_edit", "contact_edit.html", data)
}

func (h *Handlers) handleContactUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}
	c, err := parseContactForm(r)
	if err != nil {
		http.Error(w, "Invalid form data: "+err.Error(), 400)
		return
	}
	c.ID = id
	if err := UpdateContact(h.db, c); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handlers) handleContactDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}
	if err := DeleteContact(h.db, id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// --- Group handlers ---

func (h *Handlers) handleGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := GetAllGroups(h.db)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	h.renderPage(w, "groups", "groups.html", map[string]interface{}{"Groups": groups})
}

func (h *Handlers) handleGroupCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "Group name required", 400)
		return
	}
	if err := CreateGroup(h.db, name); err != nil {
		http.Error(w, "Could not create group: "+err.Error(), 500)
		return
	}
	groups, err := GetAllGroups(h.db)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	h.renderPartial(w, "group_list", map[string]interface{}{"Groups": groups})
}

func (h *Handlers) handleGroupDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}
	if err := DeleteGroup(h.db, id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	groups, err := GetAllGroups(h.db)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	h.renderPartial(w, "group_list", map[string]interface{}{"Groups": groups})
}

// --- Helpers ---

func (h *Handlers) renderPage(w http.ResponseWriter, pageName, entryTemplate string, data interface{}) {
	tmpl := h.pages[pageName]
	if tmpl == nil {
		log.Printf("template set %q not found", pageName)
		http.Error(w, "Internal server error", 500)
		return
	}
	if err := tmpl.ExecuteTemplate(w, entryTemplate, data); err != nil {
		log.Printf("template error: %v", err)
	}
}

func (h *Handlers) renderPartial(w http.ResponseWriter, name string, data interface{}) {
	tmpl := h.pages["partials"]
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("partial template error: %v", err)
		http.Error(w, "Internal server error", 500)
	}
}

func sortedGroupNames(grouped map[string][]Contact) []string {
	groupNames := make([]string, 0, len(grouped))
	for name := range grouped {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)
	for i, name := range groupNames {
		if name == "Ungrouped" {
			groupNames = append(groupNames[:i], groupNames[i+1:]...)
			groupNames = append(groupNames, "Ungrouped")
			break
		}
	}
	return groupNames
}

func parseContactForm(r *http.Request) (Contact, error) {
	if err := r.ParseForm(); err != nil {
		return Contact{}, err
	}

	firstName := strings.TrimSpace(r.FormValue("first_name"))
	lastName := strings.TrimSpace(r.FormValue("last_name"))
	if firstName == "" || lastName == "" {
		return Contact{}, fmt.Errorf("first and last name are required")
	}

	c := Contact{
		FirstName:     firstName,
		LastName:      lastName,
		FrequencyDays: 30,
	}

	if freq := r.FormValue("frequency_days"); freq != "" {
		f, err := strconv.Atoi(freq)
		if err != nil || f < 1 {
			return Contact{}, fmt.Errorf("frequency must be a positive number")
		}
		c.FrequencyDays = f
	}

	if dateStr := r.FormValue("last_contact_date"); dateStr != "" {
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return Contact{}, fmt.Errorf("invalid date format")
		}
		c.LastContactDate = sql.NullTime{Time: t, Valid: true}
	}

	if gidStr := r.FormValue("group_id"); gidStr != "" && gidStr != "0" {
		gid, err := strconv.ParseInt(gidStr, 10, 64)
		if err != nil {
			return Contact{}, fmt.Errorf("invalid group ID")
		}
		c.GroupID = sql.NullInt64{Int64: gid, Valid: true}
	}

	return c, nil
}
