package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/jung-kurt/gofpdf"
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
	// Load .env (for DATABASE_URL, SMTP settings, etc.)
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

	// NEW: send khata PDF to customer's email
	r.Post("/api/customers/{id}/send-email", sendCustomerEmailHandler)

	// Phase 3: Customer Portal APIs (ADDED)
	r.Post("/api/portal/khata-lookup", portalKhataLookupHandler)
	r.Get("/api/portal/customers/{id}/pdf", portalCustomerPDFHandler)

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

// ---------- Basic Handlers ----------

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

	customers := []Customer{} // non-nil slice

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

	entries := []Entry{} // non-nil slice

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

// ---------- Phase 2: Email with PDF ----------

// POST /api/customers/{id}/send-email
func sendCustomerEmailHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	customerID, err := strconv.Atoi(idStr)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid customer id")
		return
	}

	// 1. Get customer
	var c Customer
	err = db.QueryRow(`
		SELECT id, name, phone, email, created_at
		FROM customers
		WHERE id = $1
	`, customerID).Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.CreatedAt)
	if err == sql.ErrNoRows {
		httpError(w, http.StatusNotFound, "customer not found")
		return
	} else if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to get customer")
		return
	}

	if c.Email == "" {
		httpError(w, http.StatusBadRequest, "customer has no email")
		return
	}

	// 2. Get entries
	rows, err := db.Query(`
		SELECT id, customer_id, type, amount, note, created_at
		FROM entries
		WHERE customer_id = $1
		ORDER BY created_at ASC
	`, customerID)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to query entries")
		return
	}
	defer rows.Close()

	entries := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.CustomerID, &e.Type, &e.Amount, &e.Note, &e.CreatedAt); err != nil {
			httpError(w, http.StatusInternalServerError, "failed to scan entry")
			return
		}
		entries = append(entries, e)
	}

	// 3. Generate PDF bytes
	pdfData, err := generateKhataPDF(c, entries)
	if err != nil {
		log.Println("generate PDF error:", err)
		httpError(w, http.StatusInternalServerError, "failed to generate PDF")
		return
	}

	// 4. Prepare email text
	var totalDebit, totalCredit float64
	for _, e := range entries {
		if e.Type == "debit" {
			totalDebit += e.Amount
		} else if e.Type == "credit" {
			totalCredit += e.Amount
		}
	}
	balance := totalDebit - totalCredit

	body := fmt.Sprintf(
		"Assalam o Alaikum %s,\n\nHere is your khata summary:\n\nTotal Debit : %.2f\nTotal Credit: %.2f\nBalance     : %.2f\n\nThe detailed khata is attached as a PDF.\n\nJazakAllah,\n%s",
		c.Name,
		totalDebit,
		totalCredit,
		balance,
		getFromName(),
	)

	subject := "Your Khata Details"

	// 5. Send email with PDF attachment
	if err := sendEmailWithPDF(c.Email, subject, body, pdfData, fmt.Sprintf("khata-customer-%d.pdf", c.ID)); err != nil {
		log.Println("send email error:", err)
		// changed here: show real error message
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "email sent"})
}

// generateKhataPDF creates a simple PDF summary of the customer's khata.
func generateKhataPDF(c Customer, entries []Entry) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Title
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "Khata Summary")
	pdf.Ln(12)

	// Customer info
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 6, fmt.Sprintf("Customer: %s", c.Name))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Phone: %s", c.Phone))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Email: %s", c.Email))
	pdf.Ln(10)

	// Totals
	var totalDebit, totalCredit float64
	for _, e := range entries {
		if e.Type == "debit" {
			totalDebit += e.Amount
		} else if e.Type == "credit" {
			totalCredit += e.Amount
		}
	}
	balance := totalDebit - totalCredit

	pdf.Cell(0, 6, fmt.Sprintf("Total Debit : %.2f", totalDebit))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Total Credit: %.2f", totalCredit))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Balance     : %.2f", balance))
	pdf.Ln(10)

	// Table header
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(35, 7, "Date", "1", 0, "", false, 0, "")
	pdf.CellFormat(25, 7, "Type", "1", 0, "", false, 0, "")
	pdf.CellFormat(30, 7, "Amount", "1", 0, "", false, 0, "")
	pdf.CellFormat(100, 7, "Note", "1", 1, "", false, 0, "")

	// Table rows
	pdf.SetFont("Arial", "", 11)
	for _, e := range entries {
		dateStr := e.CreatedAt.Format("2006-01-02 15:04")
		pdf.CellFormat(35, 6, dateStr, "1", 0, "", false, 0, "")
		pdf.CellFormat(25, 6, e.Type, "1", 0, "", false, 0, "")
		pdf.CellFormat(30, 6, fmt.Sprintf("%.2f", e.Amount), "1", 0, "", false, 0, "")
		pdf.CellFormat(100, 6, e.Note, "1", 1, "", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func getFromName() string {
	name := os.Getenv("FROM_NAME")
	if name == "" {
		name = "Khata System"
	}
	return name
}

// sendEmailWithPDF sends an email with the given PDF as attachment.
func sendEmailWithPDF(to, subject, body string, pdfData []byte, filename string) error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	fromEmail := os.Getenv("FROM_EMAIL")
	fromName := getFromName()

	if smtpHost == "" || smtpPort == "" || smtpUser == "" || smtpPass == "" {
		return fmt.Errorf("SMTP configuration is missing")
	}

	if fromEmail == "" {
		fromEmail = smtpUser
	}

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)

	// Build MIME email
	var buf bytes.Buffer
	boundary := "khata-boundary-123456"

	// Headers
	buf.WriteString(fmt.Sprintf("From: %s <%s>\r\n", fromName, fromEmail))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n", boundary))
	buf.WriteString("\r\n")

	// Body part
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	buf.WriteString("Content-Transfer-Encoding: 7bit\r\n\r\n")
	buf.WriteString(body)
	buf.WriteString("\r\n")

	// Attachment part
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: application/pdf\r\n")
	buf.WriteString("Content-Transfer-Encoding: base64\r\n")
	buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", filename))

	// Base64-encode PDF data, wrap lines at 76 chars
	b64 := base64.StdEncoding.EncodeToString(pdfData)
	for i := 0; i < len(b64); i += 76 {
		end := i + 76
		if end > len(b64) {
			end = len(b64)
		}
		buf.WriteString(b64[i:end])
		buf.WriteString("\r\n")
	}

	// End boundary
	buf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	addr := smtpHost + ":" + smtpPort
	return smtp.SendMail(addr, auth, fromEmail, []string{to}, buf.Bytes())
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

// ---------- Phase 3: Customer Portal Handlers (ADDED) ----------

type portalLookupRequest struct {
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type portalKhataResponse struct {
	Customer Customer `json:"customer"`
	Totals   struct {
		Debit   float64 `json:"debit"`
		Credit  float64 `json:"credit"`
		Balance float64 `json:"balance"`
	} `json:"totals"`
	Entries []Entry `json:"entries"`
}

// POST /api/portal/khata-lookup
// Body: { "email": "...", "phone": "..." }
func portalKhataLookupHandler(w http.ResponseWriter, r *http.Request) {
	var req portalLookupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Email == "" || req.Phone == "" {
		httpError(w, http.StatusBadRequest, "email and phone are required")
		return
	}

	// 1. Find customer by email + phone
	var c Customer
	err := db.QueryRow(`
		SELECT id, name, phone, email, created_at
		FROM customers
		WHERE email = $1 AND phone = $2
		LIMIT 1
	`, req.Email, req.Phone).Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.CreatedAt)
	if err == sql.ErrNoRows {
		httpError(w, http.StatusNotFound, "no khata found for this email and phone")
		return
	} else if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to find customer")
		return
	}

	// 2. Get entries for that customer
	rows, err := db.Query(`
		SELECT id, customer_id, type, amount, note, created_at
		FROM entries
		WHERE customer_id = $1
		ORDER BY created_at ASC
	`, c.ID)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to query entries")
		return
	}
	defer rows.Close()

	entries := []Entry{}
	var totalDebit, totalCredit float64

	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.CustomerID, &e.Type, &e.Amount, &e.Note, &e.CreatedAt); err != nil {
			httpError(w, http.StatusInternalServerError, "failed to scan entry")
			return
		}
		entries = append(entries, e)

		if e.Type == "debit" {
			totalDebit += e.Amount
		} else if e.Type == "credit" {
			totalCredit += e.Amount
		}
	}

	balance := totalDebit - totalCredit

	var resp portalKhataResponse
	resp.Customer = c
	resp.Totals.Debit = totalDebit
	resp.Totals.Credit = totalCredit
	resp.Totals.Balance = balance
	resp.Entries = entries

	writeJSON(w, http.StatusOK, resp)
}

// GET /api/portal/customers/{id}/pdf
// Returns a PDF for direct download (customer portal)
func portalCustomerPDFHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	customerID, err := strconv.Atoi(idStr)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid customer id")
		return
	}

	// 1. Get customer
	var c Customer
	err = db.QueryRow(`
		SELECT id, name, phone, email, created_at
		FROM customers
		WHERE id = $1
	`, customerID).Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.CreatedAt)
	if err == sql.ErrNoRows {
		httpError(w, http.StatusNotFound, "customer not found")
		return
	} else if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to get customer")
		return
	}

	// 2. Get entries
	rows, err := db.Query(`
		SELECT id, customer_id, type, amount, note, created_at
		FROM entries
		WHERE customer_id = $1
		ORDER BY created_at ASC
	`, customerID)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to query entries")
		return
	}
	defer rows.Close()

	entries := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.CustomerID, &e.Type, &e.Amount, &e.Note, &e.CreatedAt); err != nil {
			httpError(w, http.StatusInternalServerError, "failed to scan entry")
			return
		}
		entries = append(entries, e)
	}

	// 3. Generate PDF
	pdfData, err := generateKhataPDF(c, entries)
	if err != nil {
		log.Println("portal PDF generate error:", err)
		httpError(w, http.StatusInternalServerError, "failed to generate PDF")
		return
	}

	// 4. Send PDF as download
	filename := fmt.Sprintf("khata-customer-%d.pdf", c.ID)

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfData)
}
