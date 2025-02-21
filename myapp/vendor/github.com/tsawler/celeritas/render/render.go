package render

import (
	"errors"
	"fmt"
	"github.com/CloudyKit/jet/v6"
	"github.com/alexedwards/scs/v2"
	"github.com/justinas/nosurf"
	"github.com/tsawler/celeritas/cache"
	"html/template"
	"log"
	"net/http"
	"strings"
)

// Render is the type for rendering pages
type Render struct {
	Renderer   string
	Cache      cache.Cache
	RootPath   string
	JetViews   *jet.Set
	Secure     bool
	Port       string
	ServerName string
	Session    *scs.SessionManager
}

// TemplateData defines template data
type TemplateData struct {
	IsAuthenticated bool
	IntMap          map[string]int
	StringMap       map[string]string
	FloatMap        map[string]float32
	Data            map[string]interface{}
	CSRFToken       string
	Flash           string
	Error           string
	Secure          bool
	Port            string
	ServerName      string
}

// Page renders a page using chosen templating system
func (c *Render) Page(w http.ResponseWriter, r *http.Request, view string, variables, data interface{}) error {
	switch strings.ToLower(c.Renderer) {
	case "go":
		return c.GoPage(w, r, view, data)
	case "jet":
		return c.JetPage(w, r, view, variables, data)
	default:

	}
	return errors.New("no valid renderer")
}

// GoPage renders a page using Go templates
func (c *Render) GoPage(w http.ResponseWriter, r *http.Request, view string, data interface{}) error {
	tmpl, err := template.ParseFiles(fmt.Sprintf("%s/views/%s.page.tmpl", c.RootPath, view))
	if err != nil {
		return err
	}

	td := &TemplateData{}
	if data != nil {
		td = data.(*TemplateData)
	}

	td = c.defaultData(td, r)

	err = tmpl.Funcs(functions).Execute(w, &td)

	if err != nil {
		return err
	}

	return nil
}

// any function in here is available to Go templates
var functions = template.FuncMap{}

func (c *Render) defaultData(td *TemplateData, r *http.Request) *TemplateData {
	td.CSRFToken = nosurf.Token(r)
	td.Secure = c.Secure
	td.ServerName = c.ServerName
	td.Port = c.Port
	if c.Session.Exists(r.Context(), "userID") {
		td.IsAuthenticated = true
	}
	return td
}

// addJetTemplateFunctions adds custom functions to all Jet templates
func (c *Render) addJetTemplateFunctions() {}

// JetPage renders jet templates
func (c *Render) JetPage(w http.ResponseWriter, r *http.Request, templateName string, variables, data interface{}) error {
	var vars jet.VarMap

	if variables == nil {
		vars = make(jet.VarMap)
	} else {
		vars = variables.(jet.VarMap)
	}

	// add default template data
	td := &TemplateData{}
	if data != nil {
		td = data.(*TemplateData)
	}

	td = c.defaultData(td, r)

	// add template functions
	c.addJetTemplateFunctions()

	// load the template and render it
	t, err := c.JetViews.GetTemplate(fmt.Sprintf("%s.jet", templateName))
	if err != nil {
		log.Println(err)
		return err
	}

	if err = t.Execute(w, vars, td); err != nil {
		log.Println(err)
		return err
	}
	return nil
}
