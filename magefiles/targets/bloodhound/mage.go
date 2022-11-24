package bloodhound

import (
	"context"

	"dagger.io/dagger"

	"github.com/magefile/mage/mg"

	loggerdagger "github.com/mesosphere/daggers/dagger/logger"
)

// Validate runs the bloodhound to validate the manifests.
func Validate(ctx context.Context) error {
	logger, err := loggerdagger.NewLogger(true)
	if err != nil {
		return err
	}

	client, err := dagger.Connect(ctx, dagger.WithLogOutput(logger))
	if err != nil {
		return err
	}
	defer client.Close()

	var opts []Option

	if mg.Verbose() || mg.Debug() {
		opts = append(opts, WithVerbose(4))
	}
	_, err = Run(ctx, client, client.Host().Workdir(), opts...)
	if err != nil {
		return err
	}

	return nil
}
