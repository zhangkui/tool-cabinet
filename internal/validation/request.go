package validation

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var ErrMissingField = errors.New("required field missing")
var ErrInvalidIdentifier = errors.New("identifier format invalid")
var ErrInvalidPhone = errors.New("phone format invalid")
var ErrInvalidWindow = errors.New("time window invalid")
var idPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{2,63}$`)
var phonePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

type MemberInput struct {
	ID    string
	Name  string
	Phone string
}
type ReservationInput struct {
	ToolID    string
	MemberID  string
	StartAt   time.Time
	ExpiresAt time.Time
	Note      string
}
type LoanInput struct {
	ToolID   string
	MemberID string
	Days     int
}

func ValidateMember(input MemberInput) error {
	if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Phone) == "" {
		return ErrMissingField
	}
	if !idPattern.MatchString(input.ID) {
		return ErrInvalidIdentifier
	}
	if !phonePattern.MatchString(input.Phone) {
		return ErrInvalidPhone
	}
	if len([]rune(input.Name)) < 2 {
		return errors.New("name too short")
	}
	return nil
}
func ValidateReservation(input ReservationInput, now time.Time) error {
	if !validID(input.ToolID) || !validID(input.MemberID) {
		return ErrInvalidIdentifier
	}
	if input.StartAt.Before(now.Add(-time.Minute)) {
		return ErrInvalidWindow
	}
	if input.ExpiresAt.Before(input.StartAt.Add(5 * time.Minute)) {
		return ErrInvalidWindow
	}
	if input.ExpiresAt.After(now.Add(30 * 24 * time.Hour)) {
		return ErrInvalidWindow
	}
	return nil
}
func ValidateLoan(input LoanInput) error {
	if !validID(input.ToolID) || !validID(input.MemberID) {
		return ErrInvalidIdentifier
	}
	if input.Days < 1 || input.Days > 14 {
		return errors.New("days outside allowed range")
	}
	return nil
}
func ValidateLimit(value, minimum, maximum int) error {
	if value < minimum || value > maximum {
		return errors.New("value outside allowed range")
	}
	return nil
}
func ValidateQuery(raw string) (string, error) {
	value, err := url.QueryUnescape(raw)
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if len([]rune(value)) > 100 {
		return "", errors.New("query too long")
	}
	return value, nil
}
func NormalizeNote(note string) string {
	note = strings.TrimSpace(note)
	note = strings.Join(strings.Fields(note), " ")
	if len([]rune(note)) > 200 {
		return string([]rune(note)[:200])
	}
	return note
}
func validID(value string) bool { return idPattern.MatchString(value) }
