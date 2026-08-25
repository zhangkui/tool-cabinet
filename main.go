package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"example.com/tool-cabinet/internal/cabinet"
)

type app struct{ service *cabinet.Service }
type reservationRequest struct {
	ToolID   string `json:"tool_id"`
	MemberID string `json:"member_id"`
	HoldMS   int    `json:"hold_ms"`
}
type memberRequest struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}
type loanRequest struct {
	ToolID   string `json:"tool_id"`
	MemberID string `json:"member_id"`
	Days     int    `json:"days"`
}
type returnRequest struct {
	LoanID   string `json:"loan_id"`
	MemberID string `json:"member_id"`
}
type maintenanceRequest struct {
	ToolID  string `json:"tool_id"`
	Enabled bool   `json:"enabled"`
	Actor   string `json:"actor"`
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
func (a *app) reserve(w http.ResponseWriter, r *http.Request) {
	var req reservationRequest
	if !decode(w, r, &req) || req.ToolID == "" || req.MemberID == "" {
		return
	}
	err := a.service.Reserve(r.Context(), req.MemberID, req.ToolID, time.Duration(req.HoldMS)*time.Millisecond)
	if errors.Is(err, context.Canceled) {
		http.Error(w, "reservation canceled", http.StatusRequestTimeout)
		return
	}
	if errors.Is(err, cabinet.ErrInvalidMember) {
		http.Error(w, "member inactive", http.StatusForbidden)
		return
	}
	if errors.Is(err, cabinet.ErrUnavailable) {
		http.Error(w, "tool unavailable", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *app) checkout(w http.ResponseWriter, r *http.Request) {
	var req loanRequest
	if !decode(w, r, &req) || req.ToolID == "" || req.MemberID == "" {
		return
	}
	loan, err := a.service.Checkout(req.MemberID, req.ToolID, req.Days)
	if errors.Is(err, cabinet.ErrInvalidMember) {
		http.Error(w, "member inactive", http.StatusForbidden)
		return
	}
	if errors.Is(err, cabinet.ErrUnavailable) {
		http.Error(w, "tool unavailable", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, loan)
}
func (a *app) returnLoan(w http.ResponseWriter, r *http.Request) {
	var req returnRequest
	if !decode(w, r, &req) || req.LoanID == "" {
		return
	}
	loan, err := a.service.Return(req.LoanID, req.MemberID)
	if errors.Is(err, cabinet.ErrInvalidLoan) {
		http.Error(w, "member does not own loan", http.StatusForbidden)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, loan)
}
func (a *app) registerMember(w http.ResponseWriter, r *http.Request) {
	var req memberRequest
	if !decode(w, r, &req) {
		return
	}
	if err := a.service.RegisterMember(req.ID, req.Name, req.Phone); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *app) status(w http.ResponseWriter, r *http.Request) {
	state, ok := a.service.Status(r.URL.Query().Get("tool_id"))
	if !ok {
		http.Error(w, "tool not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": state})
}
func (a *app) catalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, a.service.Catalog(r.URL.Query().Get("q")))
}
func (a *app) activeLoans(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, a.service.ActiveLoans(r.URL.Query().Get("member_id")))
}
func (a *app) overdue(w http.ResponseWriter, r *http.Request) { writeJSON(w, a.service.OverdueLoans()) }
func (a *app) maintenance(w http.ResponseWriter, r *http.Request) {
	var req maintenanceRequest
	if !decode(w, r, &req) || req.ToolID == "" {
		return
	}
	if err := a.service.SetMaintenance(req.ToolID, req.Enabled, req.Actor); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *app) metrics(w http.ResponseWriter, r *http.Request) { writeJSON(w, a.service.Metrics()) }
func (a *app) notifications(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, a.service.Notifications())
}
func (a *app) audit(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, a.service.Audit(limit))
}
func main() {
	a := &app{service: cabinet.NewService()}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /members", a.registerMember)
	mux.HandleFunc("POST /reserve", a.reserve)
	mux.HandleFunc("POST /loans", a.checkout)
	mux.HandleFunc("POST /loans/return", a.returnLoan)
	mux.HandleFunc("GET /loans/active", a.activeLoans)
	mux.HandleFunc("GET /loans/overdue", a.overdue)
	mux.HandleFunc("POST /maintenance", a.maintenance)
	mux.HandleFunc("GET /status", a.status)
	mux.HandleFunc("GET /catalog", a.catalog)
	mux.HandleFunc("GET /audit", a.audit)
	_ = http.ListenAndServe(":8080", mux)
}
