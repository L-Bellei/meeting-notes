package httpmiddleware

import "net/http"

var allowedLocalOrigins = map[string]struct{}{
	"http://localhost:34115":  {},
	"http://wails.localhost":  {},
	"https://wails.localhost": {},
}

func AllowedLocalOrigins() []string {
	origins := make([]string, 0, len(allowedLocalOrigins))
	for origin := range allowedLocalOrigins {
		origins = append(origins, origin)
	}
	return origins
}

func RejectUntrustedOrigins(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			if _, ok := allowedLocalOrigins[origin]; !ok {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
