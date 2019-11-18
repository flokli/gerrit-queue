package main

import (
	"os"

	"github.com/tweag/gerrit-queue/frontend"
	"github.com/tweag/gerrit-queue/gerrit"
	"github.com/tweag/gerrit-queue/submitqueue"

	"github.com/gin-gonic/gin"
	"github.com/urfave/cli"

	"fmt"

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

		router := gin.Default()
		frontend := frontend.MakeFrontend(router, &submitQueue)

		err = submitQueue.LoadSeries()
		if err != nil {
			log.Errorf("Error loading submit queue: %s", err)
		}

		fmt.Println()
		fmt.Println()
		fmt.Println()
		fmt.Println()
		for _, serie := range submitQueue.Series {
			fmt.Println(fmt.Sprintf("%s", serie))
			for _, changeset := range serie.ChangeSets {
				fmt.Println(fmt.Sprintf(" - %s", changeset.String()))
			}
			fmt.Println()
		}

		frontend.Run(":8080")

		if fetchOnly {
			//return backlog.Run()
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
