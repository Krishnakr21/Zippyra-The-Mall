package logger

import (
	"strings"
)

// MaskPhone masks Indian phone numbers
// Input:  +919876543210  →  Output: +91XXXXXX3210
// Input:  9876543210     →  Output: XXXXXX3210
// Input:  ""             →  Output: ***
func MaskPhone(phone string) string {
	if phone == "" {
		return "***"
	}
	if len(phone) < 4 {
		return "***"
	}
	// Keep +91 prefix if present (3 chars) and last 4 digits
	if strings.HasPrefix(phone, "+91") {
		if len(phone) <= 7 {
			return "+91****"
		}
		return "+91XXXXXX" + phone[len(phone)-4:]
	}
	return "XXXXXX" + phone[len(phone)-4:]
}

// MaskEmail masks email addresses
// Input:  krishna@gmail.com  →  Output: kr*****@gmail.com
// Input:  a@b.com            →  Output: *@b.com
func MaskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}
	user := parts[0]
	domain := parts[1]
	if len(user) == 1 {
		return "*@" + domain
	}
	if len(user) <= 2 {
		return user[:1] + "*****@" + domain
	}
	return user[:2] + "*****@" + domain
}

// MaskCardNumber masks payment card numbers
// Input:  4111111111111111  →  Output: 411111XXXXXX1111
// Input:  ""                →  Output: ***
func MaskCardNumber(card string) string {
	if card == "" || len(card) < 10 {
		return "***"
	}
	return card[:6] + "XXXXXX" + card[len(card)-4:]
}

// MaskUPI masks UPI IDs
// Input:  krishna@okicici  →  Output: kr*****@okicici
func MaskUPI(upi string) string {
	return MaskEmail(upi)
}

// MaskToken returns only last 8 chars of JWT/token
// Input:  eyJhbGci...longtoken...abc12345  →  Output: ...abc12345
// Input:  short  →  Output: ***
func MaskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return "..." + token[len(token)-8:]
}

// MaskGSTIN partially masks GSTIN
// Input:  27AAPFU0939F1ZV  →  Output: 27AAXXXXX39F1ZV
func MaskGSTIN(gstin string) string {
	if len(gstin) < 9 {
		return "***"
	}
	return gstin[:4] + "XXXXX" + gstin[len(gstin)-6:]
}

// MaskName masks customer names
// Input:  Krishna Kumar  →  Output: Kr***** Ku***
func MaskName(name string) string {
	if name == "" {
		return "***"
	}
	parts := strings.Fields(name)
	for i, part := range parts {
		if len(part) <= 2 {
			parts[i] = part[:1] + "***"
		} else {
			parts[i] = part[:2] + "*****"
		}
	}
	return strings.Join(parts, " ")
}

// LogSafeUser contains only fields safe to log.
// NEVER log raw user structs — always convert to LogSafeUser first.
type LogSafeUser struct {
	ID          string // UUID — safe
	MaskedPhone string // +91XXXXXX3210 — safe
	IsNewUser   bool   // safe
	UserType    string // CUSTOMER/STAFF/ADMIN — safe
}

// NewLogSafeUser creates a new LogSafeUser with masked PII.
func NewLogSafeUser(id, phone, userType string, isNew bool) LogSafeUser {
	return LogSafeUser{
		ID:          id,
		MaskedPhone: MaskPhone(phone),
		IsNewUser:   isNew,
		UserType:    userType,
	}
}
