package main

import (
	"os"
	"time"

	"net/http"

	"github.com/flokli/gerrit-queue/frontend"
	"github.com/flokli/gerrit-queue/gerrit"
	"github.com/flokli/gerrit-queue/misc"
	"github.com/flokli/gerrit-queue/submitqueue"

	"github.com/urfave/cli"

	"github.com/apex/log"
	"github.com/apex/log/handlers/multi"
	"github.com/apex/log/handlers/text"
)

func main() {
	var URL, username, password, projectName, branchName string
	var fetchOnly bool
	var triggerInterval int

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
		cli.IntFlag{
			Name:        "trigger-interval",
			Usage:       "How often we should trigger ourselves (interval in seconds)",
			EnvVar:      "SUBMIT_QUEUE_TRIGGER_INTERVAL",
			Destination: &triggerInterval,
			Value:       600,
		},
		cli.BoolFlag{
			Name:        "fetch-only",
			Usage:       "Only fetch changes and assemble queue, but don't actually write",
			EnvVar:      "SUBMIT_QUEUE_FETCH_ONLY",
			Destination: &fetchOnly,
		},
	}

	rotatingLogHandler := misc.NewRotatingLogHandler(10000)
	l := &log.Logger{
		Handler: multi.New(
			text.New(os.Stderr),
			rotatingLogHandler,
		),
		Level: log.DebugLevel,
	}

	app.Action = func(c *cli.Context) error {
		gerrit, err := gerrit.NewClient(l, URL, username, password, projectName, branchName)
		if err != nil {
			return err
		}
		log.Infof("Successfully connected to gerrit at %s", URL)

		runner := submitqueue.NewRunner(l, gerrit)

		handler := frontend.MakeFrontend(rotatingLogHandler, gerrit, runner)

		// fetch only on first run
		err = runner.Trigger(fetchOnly)
		if err != nil {
			log.Error(err.Error())
		}

		// ticker
		go func() {
			for {
				time.Sleep(time.Duration(triggerInterval) * time.Second)
				err = runner.Trigger(fetchOnly)
				if err != nil {
					log.Error(err.Error())
				}
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
