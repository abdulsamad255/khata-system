package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var db *sql.DB

type Customer struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Entry struct {
	ID         int       `json:"id"`
	CustomerID int       `json:"customer_id"`
	Type       string    `json:"type"` // "debit" or "credit"
	Amount     float64   `json:"amount"`
	Note       string    `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
}

func main() {
	// Load .env (for DATABASE_URL, PORT)
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("Error opening DB:", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("Error pinging DB:", err)
	}

	log.Println("Connected to database")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(corsMiddleware)

	// Routes
	r.Get("/api/health", healthHandler)

	r.Get("/api/customers", listCustomersHandler)
	r.Post("/api/customers", createCustomerHandler)

	r.Get("/api/customers/{id}", getCustomerHandler)
	r.Get("/api/customers/{id}/entries", listEntriesHandler)
	r.Post("/api/customers/{id}/entries", createEntryHandler)

	log.Println("Backend listening on port", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}

// ---------- Middleware ----------

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For dev: allow frontend at localhost:3000
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ---------- Handlers ----------

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /api/customers
func listCustomersHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT id, name, phone, email, created_at
		FROM customers
		ORDER BY created_at DESC
	`)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to query customers")
		return
	}
	defer rows.Close()

	// IMPORTANT: start with an empty slice, not nil, so JSON encodes as []
	customers := []Customer{}

	for rows.Next() {
		var c Customer
		if err := rows.Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.CreatedAt); err != nil {
			httpError(w, http.StatusInternalServerError, "failed to scan customer")
			return
		}
		customers = append(customers, c)
	}

	writeJSON(w, http.StatusOK, customers)
}

// POST /api/customers
// JSON body: { "name": "...", "phone": "...", "email": "..." }
func createCustomerHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if input.Name == "" {
		httpError(w, http.StatusBadRequest, "name is required")
		return
	}

	var id int
	var created time.Time
	err := db.QueryRow(`
		INSERT INTO customers (name, phone, email)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`, input.Name, input.Phone, input.Email).Scan(&id, &created)

	if err != nil {
		log.Println("insert customer error:", err)
		httpError(w, http.StatusInternalServerError, "failed to insert customer")
		return
	}

	customer := Customer{
		ID:        id,
		Name:      input.Name,
		Phone:     input.Phone,
		Email:     input.Email,
		CreatedAt: created,
	}

	writeJSON(w, http.StatusCreated, customer)
}

// GET /api/customers/{id}
func getCustomerHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid customer id")
		return
	}

	var c Customer
	err = db.QueryRow(`
		SELECT id, name, phone, email, created_at
		FROM customers
		WHERE id = $1
	`, id).Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.CreatedAt)
	if err == sql.ErrNoRows {
		httpError(w, http.StatusNotFound, "customer not found")
		return
	} else if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to get customer")
		return
	}

	writeJSON(w, http.StatusOK, c)
}

// GET /api/customers/{id}/entries
func listEntriesHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	customerID, err := strconv.Atoi(idStr)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid customer id")
		return
	}

	rows, err := db.Query(`
		SELECT id, customer_id, type, amount, note, created_at
		FROM entries
		WHERE customer_id = $1
		ORDER BY created_at DESC
	`, customerID)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to query entries")
		return
	}
	defer rows.Close()

	// IMPORTANT: start with empty slice, not nil
	entries := []Entry{}

	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.CustomerID, &e.Type, &e.Amount, &e.Note, &e.CreatedAt); err != nil {
			httpError(w, http.StatusInternalServerError, "failed to scan entry")
			return
		}
		entries = append(entries, e)
	}

	writeJSON(w, http.StatusOK, entries)
}

// POST /api/customers/{id}/entries
// JSON body: { "type": "debit" | "credit", "amount": 1000, "note": "..." }
func createEntryHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	customerID, err := strconv.Atoi(idStr)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid customer id")
		return
	}

	var input struct {
		Type   string  `json:"type"`
		Amount float64 `json:"amount"`
		Note   string  `json:"note"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if input.Type != "debit" && input.Type != "credit" {
		httpError(w, http.StatusBadRequest, "type must be 'debit' or 'credit'")
		return
	}

	if input.Amount <= 0 {
		httpError(w, http.StatusBadRequest, "amount must be > 0")
		return
	}

	var id int
	var created time.Time
	err = db.QueryRow(`
		INSERT INTO entries (customer_id, type, amount, note)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`, customerID, input.Type, input.Amount, input.Note).Scan(&id, &created)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to insert entry")
		return
	}

	entry := Entry{
		ID:         id,
		CustomerID: customerID,
		Type:       input.Type,
		Amount:     input.Amount,
		Note:       input.Note,
		CreatedAt:  created,
	}

	writeJSON(w, http.StatusCreated, entry)
}

// ---------- Helpers ----------

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func httpError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
