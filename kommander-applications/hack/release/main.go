package main

import (
	"context"

	"github.com/mesosphere/kommander-applications/hack/release/cmd"
)

func main() {
	cmd.Execute(context.Background())
}
