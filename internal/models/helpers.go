package models

import (
	"html"
	"strconv"
	"time"
)

// esc HTML-escapes s unless raw is true.
func Esc(s string, raw bool) string {
	if raw {
		return s
	}
	return html.EscapeString(s)
}

// TimeStr formats a *time.Time as "2006-01-02 15:04:05", or "None" if nil.
func TimeStr(t *time.Time) string {
	if t == nil {
		return "None"
	}
	return t.Format("2006-01-02 15:04:05")
}

// TimeStrLocal formats a *time.Time for use in datetime-local inputs, or "" if nil.
func TimeStrLocal(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02T15:04")
}

// BoolPtr returns a pointer to the given bool.
func BoolPtr(b bool) *bool {
	return &b
}

// NowPtr returns a pointer to the current UTC time.
func NowPtr() *time.Time {
	t := time.Now().UTC()
	return &t
}

// FormatFloat formats a float64 to 2 decimal places.
func FormatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}
