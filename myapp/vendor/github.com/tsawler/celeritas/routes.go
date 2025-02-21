package celeritas

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"net/http"
)

// routes returns routes (an http.handler) consisting of default
// middleware. Do not place any actual routes in here, since
// the end user may wish to add their own middleware.
func (c *Celeritas) routes() http.Handler {
	mux := chi.NewRouter()
	mux.Use(middleware.RequestID)
	mux.Use(middleware.RealIP)
	if c.Debug {
		mux.Use(middleware.Logger)
	}
	mux.Use(middleware.Recoverer)
	mux.Use(c.SessionLoad)
	mux.Use(c.NoSurf)
	mux.Use(c.CheckForMaintenanceMode)

	return mux
}
