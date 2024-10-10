package extraimages

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/hack/release/pkg/constants"
)

func UpdateExtraImagesVersions(kommanderApplicationsRepo, chartVersion string) error {
	path := filepath.Join(
		kommanderApplicationsRepo,
		constants.KommanderAppPath,
		"*",
		"extra-images.txt",
	)

	matches, err := filepath.Glob(path)
	if err != nil {
		return err
	}

	if len(matches) == 0 {
		return fmt.Errorf("no matches found for extra images path, (verify the kommander-applications repo path is correct): %s", path)
	}
	if len(matches) > 1 {
		return fmt.Errorf("found > 1 match for extra images path (there should only be one match): %s", path)
	}

	if err := os.WriteFile(
		matches[0],
		[]byte(fmt.Sprintf("mesosphere/kommander-applications-server:%s\n", chartVersion)),
		0o644,
	); err != nil {
		return fmt.Errorf("error while updating extra-images file: %w", err)
	}

	return nil
}
