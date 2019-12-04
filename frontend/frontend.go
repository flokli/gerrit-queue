package frontend

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"html/template"

	"github.com/gin-gonic/gin"
	"github.com/rakyll/statik/fs"

	"github.com/tweag/gerrit-queue/gerrit"
	"github.com/tweag/gerrit-queue/misc"
	_ "github.com/tweag/gerrit-queue/statik" // register static assets
	"github.com/tweag/gerrit-queue/submitqueue"
)

//loadTemplate loads a list of templates, relative to the statikFS root, and a FuncMap, and returns a template object
func loadTemplate(templateNames []string, funcMap template.FuncMap) (*template.Template, error) {
	if len(templateNames) == 0 {
		return nil, fmt.Errorf("templateNames can't be empty")
	}
	tmpl := template.New(templateNames[0]).Funcs(funcMap)
	statikFS, err := fs.New()
	if err != nil {
		return nil, err
	}

	for _, templateName := range templateNames {
		r, err := statikFS.Open("/" + templateName)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		contents, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, err
		}
		tmpl, err = tmpl.Parse(string(contents))
		if err != nil {
			return nil, err
		}
	}

	return tmpl, nil
}

// MakeFrontend returns a http.Handler
func MakeFrontend(rotatingLogHandler *misc.RotatingLogHandler, gerritClient *gerrit.Client, runner *submitqueue.Runner) http.Handler {
	router := gin.Default()

	projectName := gerritClient.GetProjectName()
	branchName := gerritClient.GetBranchName()

	router.GET("/", func(c *gin.Context) {
		var wipSerie *gerrit.Serie = nil
		HEAD := ""
		currentlyRunning := runner.IsCurrentlyRunning()

		// don't trigger operations requiring a lock
		if !currentlyRunning {
			wipSerie = runner.GetWIPSerie()
			HEAD = gerritClient.GetHEAD()
		}

		funcMap := template.FuncMap{
			"changesetURL": func(changeset *gerrit.Changeset) string {
				return gerritClient.GetChangesetURL(changeset)
			},
		}

		tmpl := template.Must(loadTemplate([]string{
			"index.tmpl.html",
			"serie.tmpl.html",
			"changeset.tmpl.html",
		}, funcMap))

		tmpl.ExecuteTemplate(c.Writer, "index.tmpl.html", gin.H{
			// Config
			"projectName": projectName,
			"branchName":  branchName,

			// State
			"currentlyRunning": currentlyRunning,
			"wipSerie":         wipSerie,
			"HEAD":             HEAD,

			// History
			"memory": rotatingLogHandler,
		})
	})
	return router
}
