package celeritas

import (
	"github.com/justinas/nosurf"
	"net/http"
	"os"
)

// CreateDirIfNotExist creates a directory (path) if it is not already there
func (c *Celeritas) CreateDirIfNotExist(path string) error {
	const mode = 0755
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.Mkdir(path, mode)
		if err != nil {
			c.InfoLog.Println(err)
			return err
		}
	}
	return nil
}

// CreateFileIfNotExists creates a file at path if it does not exist already
func (c *Celeritas) CreateFileIfNotExists(path string) error {
	var _, err = os.Stat(path)
	if os.IsNotExist(err) {
		var file, err = os.Create(path)
		if err != nil {
			return err
		}

		defer func(file *os.File) {
			_ = file.Close()
		}(file)
	}
	return nil
}

// VerifyCSRF is used to verify csrf tokens for /api routes,
// when the token is not sent as a post value
func (c *Celeritas) VerifyCSRF(r *http.Request, token string) bool {
	if !nosurf.VerifyToken(nosurf.Token(r), token) {
		return false
	}
	return true
}
