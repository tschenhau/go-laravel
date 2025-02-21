package middleware

import (
	"myapp/data"

	"github.com/tschenhau/celeritas"
)

type Middleware struct {
	App    *celeritas.Celeritas
	Models data.Models
}
