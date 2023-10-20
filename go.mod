module github.com/flokli/gerrit-queue

go 1.16

require (
	github.com/andygrunwald/go-gerrit v0.0.0-20190825170856-5959a9bf9ff8
	github.com/apex/log v1.1.1
	github.com/stretchr/testify v1.7.0
	github.com/urfave/cli v1.22.1
)

replace github.com/andygrunwald/go-gerrit => github.com/lukegb/go-gerrit v0.0.0-20231016235128-b5317f06cc92
