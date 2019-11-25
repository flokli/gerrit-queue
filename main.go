//go:generate statik -f

package main

import (
	"os"
	"time"

	"net/http"

	"github.com/tweag/gerrit-queue/frontend"
	"github.com/tweag/gerrit-queue/gerrit"
	"github.com/tweag/gerrit-queue/submitqueue"

	"github.com/urfave/cli"

	log "github.com/sirupsen/logrus"
)

func main() {
	// configure logging
	log.SetFormatter(&log.TextFormatter{})
	//log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	var URL, username, password, projectName, branchName, submitQueueTag string
	var fetchOnly bool

	app := cli.NewApp()
	app.Name = "gerrit-queue"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "url",
			Usage:       "URL to the gerrit instance",
			EnvVar:      "GERRIT_URL",
			Destination: &URL,
			Required:    true,
		},
		cli.StringFlag{
			Name:        "username",
			Usage:       "Username to use to login to gerrit",
			EnvVar:      "GERRIT_USERNAME",
			Destination: &username,
			Required:    true,
		},
		cli.StringFlag{
			Name:        "password",
			Usage:       "Password to use to login to gerrit",
			EnvVar:      "GERRIT_PASSWORD",
			Destination: &password,
			Required:    true,
		},
		cli.StringFlag{
			Name:        "project",
			Usage:       "Gerrit project name to run the submit queue for",
			EnvVar:      "GERRIT_PROJECT",
			Destination: &projectName,
			Required:    true,
		},
		cli.StringFlag{
			Name:        "branch",
			Usage:       "Destination branch",
			EnvVar:      "GERRIT_BRANCH",
			Destination: &branchName,
			Value:       "master",
		},
		cli.StringFlag{
			Name:        "submit-queue-tag",
			Usage:       "the tag used to submit something to the submit queue",
			EnvVar:      "SUBMIT_QUEUE_TAG",
			Destination: &submitQueueTag,
			Value:       "submit_me",
		},
		cli.BoolFlag{
			Name:        "fetch-only",
			Usage:       "Only fetch changes and assemble queue, but don't actually write",
			EnvVar:      "SUBMIT_QUEUE_FETCH_ONLY",
			Destination: &fetchOnly,
		},
	}

	app.Action = func(c *cli.Context) error {
		gerritClient, err := gerrit.NewClient(URL, username, password)
		if err != nil {
			return err
		}
		log.Printf("Successfully connected to gerrit at %s", URL)

		submitQueue := submitqueue.MakeSubmitQueue(gerritClient, projectName, branchName, submitQueueTag)
		runner := submitqueue.NewRunner(submitQueue)

		handler := frontend.MakeFrontend(runner)

		// fetch only on first run
		runner.Trigger(true)

		// ticker
		go func() {
			for {
				time.Sleep(time.Minute * 10)
				runner.Trigger(fetchOnly)
			}
		}()

		server := http.Server{
			Addr:    ":8080",
			Handler: handler,
		}

		server.ListenAndServe()
		if err != nil {
			log.Fatal(err)
		}

		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

	// mux := http.NewServeMux()

	// options := &gerrit.EventsLogOptions{}
	// events, _, _, err := gerritClient.EventsLog.GetEvents(options)

	// TODOS:
	// - create submit queue user
	// - handle event log, either by accepting webhooks, or by streaming events?

	//n := negroni.Classic()
	//n.UseHandler(mux)

	//fmt.Println("Listening on :3000…")
	//http.ListenAndServe(":3000", n)
}
