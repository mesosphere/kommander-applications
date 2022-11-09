package appversion

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mesosphere/kommander-applications/hack/release/pkg/constants"
)

var (
	ErrVersionNotFound = errors.New("cannot detect existing kommander app version")

	// kommander-0.4.0-d2iq-defaults
	// kommander-0.4.0-overrides
	varNames = regexp.MustCompile(
		`(?P<prefix>kommander(-appmanagement)?-)` +
			constants.SemverRegexp +
			`(?P<suffix>(-d2iq-defaults)|(-overrides))`,
	)
)

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

	if oldVersion == version {
		// nothing to do
		return nil
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

// ReplaceContent replaces version inside files
func ReplaceContent(ctx context.Context, dir, version string) (int, error) {
	kommanderPath := filepath.Clean(constants.KommanderAppPath)

	changes := 0
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, _ error) error {
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		if d.IsDir() {
			if !strings.HasPrefix(kommanderPath, relPath) && !strings.HasPrefix(relPath, kommanderPath) {
				return filepath.SkipDir
			}
		}

		if !d.Type().IsRegular() || filepath.Ext(d.Name()) != ".yaml" {
			return nil
		}

		newChanges, err := replaceContentInFile(ctx, path, version)
		changes += newChanges

		return err
	})

	return changes, err
}

func replaceContentInFile(ctx context.Context, file string, version string) (int, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	fscanner := bufio.NewScanner(f)
	output := &bytes.Buffer{}

	changes := 0
	for fscanner.Scan() {
		text := fscanner.Text()

		newText := varNames.ReplaceAllStringFunc(text, func(match string) string {
			newValue, ok := replaceInString(match, version)
			if ok {
				changes++
				return newValue
			}
			return match
		})

		fmt.Fprintln(output, newText)
	}

	f.Close()

	if changes > 0 {
		f, err = os.Create(file)
		if err != nil {
			return 0, err
		}
		defer f.Close()

		_, err = io.Copy(f, output)
		return changes, err
	}

	return changes, nil
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

func replaceInString(input, replace string) (string, bool) {
	prefixIndex := varNames.SubexpIndex("prefix")
	suffixIndex := varNames.SubexpIndex("suffix")

	match := varNames.FindStringSubmatch(input)
	if len(match) < suffixIndex {
		return "", false
	}

	return match[prefixIndex] + replace + match[suffixIndex], true
}
