package upgradematrix

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

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

	err = os.WriteFile(filepath.Join(kommanderApplicationsRepo, "upgrade-matrix.yaml"), output, 0644)
	if err != nil {
		log.Print("cannot write upgrade-matrix.yaml")
		return err
	}

	return nil
}
