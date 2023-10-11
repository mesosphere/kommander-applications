package main

import (
	"log"

	"github.com/mesosphere/kommander-applications/apptests/cmd"
)

func main() {
	if err := cmd.NewCommand().Execute(); err != nil {
		log.Fatal(err)
	}
}
