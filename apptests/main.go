package main

import (
	"log"
	"os"

	"github.com/mesosphere/kommander-applications/apptests/cmd"
)

func main() {
	if err := cmd.NewCommand(os.Stdout).Execute(); err != nil {
		log.Fatal(err)
	}
}
