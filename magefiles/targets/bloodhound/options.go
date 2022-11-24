package bloodhound

import (
	"github.com/caarlos0/env/v6"
)

type config struct {
	BloodHoundVersion string   `env:"VERSION,notEmpty" envDefault:"0.8.1"`
	Args              []string `env:"ARGS" envSeparator:" "`
	Verbose           int      `env:"VERBOSE"`
}

func loadConfigFromEnv() (config, error) {
	cfg := config{}

	if err := env.Parse(&cfg, env.Options{Prefix: "BLOODHOUND_"}); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// Option is a function that configures the precommit checks.
type Option func(config) config

// WithBloodHoundVersion sets the bloodhound cli version to use for the container.
func WithBloodHoundVersion(version string) Option {
	return func(c config) config {
		c.BloodHoundVersion = version
		return c
	}
}

// WithArgs sets the arguments to pass to github cli.
func WithArgs(args ...string) Option {
	return func(c config) config {
		c.Args = args
		return c
	}
}

// WithVerbose sets the verbose level to use for the container.
func WithVerbose(verbose int) Option {
	return func(c config) config {
		c.Verbose = verbose
		return c
	}
}
