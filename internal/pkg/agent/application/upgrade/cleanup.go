// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package upgrade

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/elastic-agent/internal/pkg/agent/application/paths"
	"github.com/elastic/elastic-agent/pkg/core/logger"
)

// cleanNonMatchingVersionsFromDownloads will remove files that do not have the passed version number from the downloads directory.
func cleanNonMatchingVersionsFromDownloads(log *logger.Logger, version string) error {
	downloadsPath := paths.Downloads()
	log.Infow("Cleaning up non-matching downloaded versions", "version", version, "downloads.path", downloadsPath)

	files, err := os.ReadDir(downloadsPath)
	if os.IsNotExist(err) {
		// nothing to clean up
		return nil
	}

	if err != nil {
		return fmt.Errorf("unable to read directory %q: %w", paths.Downloads(), err)
	}
	var errs []error
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !strings.Contains(file.Name(), version) {
			if err := os.Remove(filepath.Join(paths.Downloads(), file.Name())); err != nil {
				errs = append(errs, fmt.Errorf("unable to remove file %q: %w", filepath.Join(paths.Downloads(), file.Name()), err))
			}
		}
	}
	return errors.Join(errs...)
}
