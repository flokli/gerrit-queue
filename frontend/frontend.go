package frontend

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"encoding/json"

	"html/template"

	"github.com/rakyll/statik/fs"

	"github.com/apex/log"

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
	projectName := gerritClient.GetProjectName()
	branchName := gerritClient.GetBranchName()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
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
			"levelToClasses": func(level log.Level) string {
				switch level {
				case log.DebugLevel:
					return "text-muted"
				case log.InfoLevel:
					return "text-info"
				case log.WarnLevel:
					return "text-warning"
				case log.ErrorLevel:
					return "text-danger"
				case log.FatalLevel:
					return "text-danger"
				default:
					return "text-white"
				}
			},
			"fieldsToJSON": func(fields log.Fields) string {
				jsonData, _ := json.Marshal(fields)
				return string(jsonData)
			},
		}

		tmpl := template.Must(loadTemplate([]string{
			"index.tmpl.html",
			"serie.tmpl.html",
			"changeset.tmpl.html",
		}, funcMap))

		tmpl.ExecuteTemplate(w, "index.tmpl.html", map[string]interface{}{
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
	return mux
}
