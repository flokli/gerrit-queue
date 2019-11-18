package frontend

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tweag/gerrit-queue/submitqueue"
)

// Frontend holds a gin Engine and the Sergequeue object
type Frontend struct {
	Router      *gin.Engine
	SubmitQueue *submitqueue.SubmitQueue
}

// MakeFrontend configures the router and returns a new Frontend struct
func MakeFrontend(router *gin.Engine, submitQueue *submitqueue.SubmitQueue) *Frontend {
	// FIXME: use go generators and statik
	router.LoadHTMLGlob("templates/*")
	router.GET("/submit-queue.json", func(c *gin.Context) {
		// FIXME: do this periodically
		err := submitQueue.UpdateHEAD()
		if err != nil {
			c.AbortWithError(http.StatusBadGateway, fmt.Errorf("unable to update HEAD"))
		}
		c.JSON(http.StatusOK, submitQueue)
	})

	router.GET("/", func(c *gin.Context) {
		// FIXME: do this periodically
		// TODO: add hyperlinks to changesets
		err := submitQueue.UpdateHEAD()
		if err != nil {
			c.AbortWithError(http.StatusBadGateway, fmt.Errorf("unable to update HEAD"))
		}
		c.HTML(http.StatusOK, "submit-queue.tmpl.html", gin.H{
			"series":      submitQueue.Series,
			"projectName": submitQueue.ProjectName,
			"branchName":  submitQueue.BranchName,
			"HEAD":        submitQueue.HEAD,
		})
	})
	return &Frontend{
		Router:      router,
		SubmitQueue: submitQueue,
	}
}

// Run starts the webserver on a given address
func (f *Frontend) Run(addr string) error {
	return f.Router.Run(addr)
}
