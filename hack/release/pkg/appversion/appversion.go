package appversion

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/hack/release/pkg/constants"
)

var ErrVersionNotFound = errors.New("cannot detect existing kommander app version")

func SetKommanderAppsVersion(ctx context.Context, dir string, version string) error {
	kommanderPath := filepath.Join(dir, constants.KommanderAppPath)
	dirs, err := os.ReadDir(kommanderPath)
	if err != nil {
		return err
	}

	var oldVersion string
	for _, d := range dirs {
		if d.IsDir() {
			oldVersion = d.Name()
			break
		}
	}

	if oldVersion == "" {
		return ErrVersionNotFound
	}

	for _, componentDir := range []string{constants.KommanderAppPath, constants.KommanderAppMgmtPath} {
		if err := move(ctx,
			dir,
			filepath.Join(componentDir, oldVersion),
			filepath.Join(componentDir, version),
		); err != nil {
			return fmt.Errorf("error while moving directories: %w", err)
		}
	}

	return nil
}

func move(ctx context.Context, dir, oldVersion, newVersion string) error {
	cmd := exec.CommandContext(ctx,
		"git",
		"mv",
		oldVersion,
		newVersion,
	)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("git mv command failed: %s", string(output))
		return err
	}

	return nil
}
