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

	"github.com/apex/log"
	"github.com/apex/log/handlers/memory"
	"github.com/apex/log/handlers/multi"
	"github.com/apex/log/handlers/text"
)

func main() {
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

	memoryLogHandler := memory.New()
	l := &log.Logger{
		Handler: multi.New(
			text.New(os.Stderr),
			memoryLogHandler,
		),
		Level: log.DebugLevel,
	}

	app.Action = func(c *cli.Context) error {
		gerrit, err := gerrit.NewClient(l, URL, username, password, projectName, branchName)
		if err != nil {
			return err
		}
		log.Infof("Successfully connected to gerrit at %s", URL)

		runner := submitqueue.NewRunner(l, gerrit, submitQueueTag)

		handler := frontend.MakeFrontend(memoryLogHandler, gerrit, runner)

		// fetch only on first run
		runner.Trigger(fetchOnly)

		// ticker
		go func() {
			for {
				time.Sleep(time.Minute * 5)
				runner.Trigger(fetchOnly)
			}
		}()

		server := http.Server{
			Addr:    ":8080",
			Handler: handler,
		}

		server.ListenAndServe()
		if err != nil {
			log.Fatalf(err.Error())
		}

		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err.Error())
	}

	// TODOS:
	// - handle event log, either by accepting webhooks, or by streaming events?
}
