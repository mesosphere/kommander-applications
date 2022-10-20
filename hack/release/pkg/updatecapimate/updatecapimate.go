package updatecapimate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mesosphere/kommander-applications/hack/release/pkg/constants"
)

func UpdateCAPIMateVersion(kommanderApplicationsRepo, containerImageVersion string) error {
	pathToCM := filepath.Join(constants.KommanderAppPath, "*/*/cm.yaml")
	matches, err := filepath.Glob(filepath.Join(kommanderApplicationsRepo, pathToCM))
	if err != nil {
		return err
	}
	if len(matches) != 1 {
		return fmt.Errorf("incorrect number of matches found. There should be 1 match. %s (verify the kommander-applications repo path is correct)", pathToCM)
	}

	chartFilePath := matches[0]

	read, err := os.ReadFile(chartFilePath)
	if err != nil {
		return err
	}
	newContents := strings.Replace(string(read), constants.CAPIMateDefaultVersion, containerImageVersion, -1)

	err = os.WriteFile(chartFilePath, []byte(newContents), 0)
	if err != nil {
		return err
	}

  return nil
}
