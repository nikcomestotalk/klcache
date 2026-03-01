package server

import (
	"net/http"
	"strings"
)

// AuthMiddleware ensures that requests either:
// 1. Originate from the loopback interface (localhost) OR
// 2. Contain the correct Bearer auth token if one is configured
func AuthMiddleware(authToken string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if it's a localhost request
		remoteAddr := r.RemoteAddr
		if strings.HasPrefix(remoteAddr, "127.0.0.1:") || strings.HasPrefix(remoteAddr, "[::1]:") {
			// Allow local requests without token enforcement (optional, but requested for tight coupling)
			// Actually, if they want tight coupling, they might want localhost to require token too,
			// but we will enforce token if it exists and wasn't local.
			// Let's enforce token ALWAYS if configured, except for internal cluster proxying which we can assume will pass tokens.
			// The requirements say: "Only allow requests originating from 127.0.0.1 or the specified APP_AUTH_TOKEN"
		}

		// If an auth token is configured, let's verify it
		if authToken != "" {
			// Read from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// No token provided. If the request is from localhost we allow it based on the rule
				if !isLocalHost(remoteAddr) {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
			} else {
				// Token provided, verify it
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) != 2 || parts[0] != "Bearer" || parts[1] != authToken {
					http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	}
}

func isLocalHost(remoteAddr string) bool {
	return strings.HasPrefix(remoteAddr, "127.0.0.1:") || strings.HasPrefix(remoteAddr, "[::1]:")
}
