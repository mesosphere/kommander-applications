package upgradematrix

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	upgradeMatrixEnv  = "NKP_RELEASE_AUTOMATION_UPGRADE_MATRIX"
	upgradeMatrixFile = "upgrade-matrix.yaml"
)

var ErrIncorrectFormat = errors.New("upgrade matrix does not appear to be in the correct format, unable to update upgrade-matrix.yaml")

// UpdateUpgradeMatrix updates the upgrade-matrix.yaml file in the kommander-applications repository with the contents \
// of the upgradeMatrixEnv environment variable.
func UpdateUpgradeMatrix(ctx context.Context, kommanderApplicationsRepo string) error {
	// Get the upgrade matrix environment variable
	upgradeMatrix := os.Getenv(upgradeMatrixEnv)
	if upgradeMatrix == "" {
		log.Printf("upgrade matrix environment variable %s is empty, unable to update upgrade-matrix.yaml", upgradeMatrixEnv)
		return nil
	}

	// Check that upgradeMatrix appears to be correct
	if !strings.HasPrefix(upgradeMatrix, "upgrades:") {
		return ErrIncorrectFormat
	}

	// Write the upgrade matrix to the file
	err := os.WriteFile(filepath.Join(kommanderApplicationsRepo, upgradeMatrixFile), []byte(upgradeMatrix), 0644)
	if err != nil {
		log.Print("cannot write upgrade-matrix.yaml")
		return err
	}

	log.Print("Updated upgrade matrix")

	return nil
}
