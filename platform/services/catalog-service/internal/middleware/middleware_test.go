package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequireCustomer(t *testing.T) {
	middleware := RequireCustomer()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name           string
		userType       string
		expectedStatus int
	}{
		{
			name:           "customer access allowed",
			userType:       "customer",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "staff access denied",
			userType:       "staff",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "no user type denied",
			userType:       "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.userType != "" {
				req.Header.Set("X-User-Type", tt.userType)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestRequireStaff(t *testing.T) {
	middleware := RequireStaff()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name           string
		userType       string
		expectedStatus int
	}{
		{
			name:           "staff access allowed",
			userType:       "staff",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "customer access denied",
			userType:       "customer",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "no user type denied",
			userType:       "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.userType != "" {
				req.Header.Set("X-User-Type", tt.userType)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
