// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"os"
	"path/filepath"
)

// ScanAndRegister walks the applications/ directory under repoRoot and calls
// RegisterDefaultTests for every application that is not already in
// registeredLabels. This gives every catalog app an install + upgrade
// template test automatically, while still allowing custom _test.go files
// to override specific apps.
//
// Pass nil or an empty map for registeredLabels to auto-register all apps.
func ScanAndRegister(repoRoot string, registeredLabels map[string]bool) {
	appsDir := filepath.Join(repoRoot, "applications")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		appName := entry.Name()
		if registeredLabels[appName] {
			continue
		}
		RegisterDefaultTests(appName)
	}
}
