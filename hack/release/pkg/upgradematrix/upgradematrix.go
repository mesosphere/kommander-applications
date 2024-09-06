package upgradematrix

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var ErrDKPNotFound = errors.New("gh dkp command not found")

func UpdateUpgradeMatrix(ctx context.Context, kommanderApplicationsRepo string) error {
	cmd := exec.CommandContext(ctx,
		"bash",
		"-c",
		"gh dkp generate upgrade-matrix --json | yq -p json -o yaml",
	)
	cmd.Dir = kommanderApplicationsRepo

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("gh dkp generate upgrade-matrix command failed: %s", string(output))
		return err
	}

	// Check output to determine if dkp extension is installed
	if strings.Contains(string(output), "unknown command \"dkp") {
		return ErrDKPNotFound
	}

	err = os.WriteFile(filepath.Join(kommanderApplicationsRepo, "upgrade-matrix.yaml"), output, 0644)
	if err != nil {
		log.Print("cannot write upgrade-matrix.yaml")
		return err
	}

	return nil
}
