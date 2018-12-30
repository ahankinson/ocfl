package main

import (
	"log"
	"os"

	"github.com/urfave/cli"
)

var mainOpts = struct {
	root string
	user string
}{}

func main() {
	app := cli.NewApp()
	app.Name = "ocfl"
	app.Usage = "OCFL commandline utilities"
	app.EnableBashCompletion = true
	app.Commands = []cli.Command{
		ls,
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "root, r",
			Usage:       "OCFL root (uri or file)",
			EnvVar:      "OCFL_ROOT",
			Destination: &mainOpts.root,
		},
		cli.StringFlag{
			Name:        "user, u",
			Usage:       "OCFL user",
			EnvVar:      "OCFL_USER",
			Destination: &mainOpts.user,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
