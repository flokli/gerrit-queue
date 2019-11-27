package frontend

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"html/template"

	"github.com/gin-gonic/gin"
	"github.com/rakyll/statik/fs"

	"github.com/tweag/gerrit-queue/gerrit"
	_ "github.com/tweag/gerrit-queue/statik" // register static assets
	"github.com/tweag/gerrit-queue/submitqueue"
)

//TODO: log last update

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

// MakeFrontend configures the router and returns a new Frontend struct
func MakeFrontend(runner *submitqueue.Runner) http.Handler {
	router := gin.Default()

	router.GET("/submit-queue.json", func(c *gin.Context) {
		submitQueue, _, _ := runner.GetState()
		c.JSON(http.StatusOK, submitQueue)
	})

	router.GET("/", func(c *gin.Context) {
		submitQueue, currentlyRunning, results := runner.GetState()

		funcMap := template.FuncMap{
			"isAutoSubmittable": func(serie *submitqueue.Serie) bool {
				return submitQueue.IsAutoSubmittable(serie)
			},
			"changesetURL": func(changeset *gerrit.Changeset) string {
				return submitQueue.GetChangesetURL(changeset)
			},
		}

		tmpl := template.Must(loadTemplate([]string{"submit-queue.tmpl.html", "changeset.tmpl.html"}, funcMap))

		tmpl.ExecuteTemplate(c.Writer, "submit-queue.tmpl.html", gin.H{
			"series":      submitQueue.Series,
			"projectName": submitQueue.ProjectName,
			"branchName":  submitQueue.BranchName,
			"HEAD":        submitQueue.HEAD,
			"currentlyRunning": currentlyRunning,
			"results": results,
		})
	})
	return router
}
