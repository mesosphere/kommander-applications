package cli

import (
	"github.com/spf13/pflag"
)

type Settings struct {
	Applications []string
}

func New() *Settings {
	return &Settings{Applications: make([]string, 0)}
}

func (s *Settings) AddFlags(flg *pflag.FlagSet) {
	flg.StringArrayVarP(&s.Applications, "applications", "a", s.Applications, "comma-separated list of application to test")
}
