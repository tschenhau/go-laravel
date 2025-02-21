package celeritas

import (
	"fmt"
	"github.com/justinas/nosurf"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// SessionLoad takes care of session data on every request
func (c *Celeritas) SessionLoad(next http.Handler) http.Handler {
	return c.Session.LoadAndSave(next)
}

// NoSurf implements CSRF protection
func (c *Celeritas) NoSurf(next http.Handler) http.Handler {
	csrfHandler := nosurf.New(next)
	secure, _ := strconv.ParseBool(c.config.cookie.secure)
	csrfHandler.ExemptGlob("/api/*")
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Domain:   c.config.cookie.domain,
	})

	return csrfHandler
}

// CheckForMaintenanceMode checks for the presence of a file named maintenance
// in the tmp directory. If it exists, we return http status 503 and display
// a message indicating that the server is under maintenance
func (c *Celeritas) CheckForMaintenanceMode(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(fmt.Sprintf("%s/tmp/maintenance", c.RootPath)); err == nil {
			if !strings.Contains(r.URL.Path, "/public/maintenance.html") && !strings.Contains(r.URL.Path, "/public/images/") {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Header().Set("Retry-After:", "300")
				w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, post-check=0, pre-check=0")
				http.ServeFile(w, r, fmt.Sprintf("%s/public/maintenance.html", c.RootPath))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
