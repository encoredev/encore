package compiler

import (
	"io/fs"
	"path/filepath"
	"strings"

	"encr.dev/compiler/internal/cueutil"
	"encr.dev/parser/est"
	"encr.dev/pkg/eerror"
	"encr.dev/pkg/vfs"
)

// pickupConfigFiles pulls any user configuration files off the filesystem
// and into a virtual file system, which then can be used to generate the runtime
// configuration for each service
func (b *builder) pickupConfigFiles() error {
	// Create a virtual filesystem for the config files
	configFiles, err := vfs.FromDir(b.appRoot, func(path string, info fs.DirEntry) bool {
		// any CUE files
		if filepath.Ext(path) == ".cue" {
			return true
		}

		// Pickup any files within a CUE module folder (either at the root of the app or in a subfolder)
		if strings.Contains(path, "/cue.mod/") || strings.HasPrefix(path, "cue.mod/") {
			return true
		}
		return false
	})
	if err != nil {
		return eerror.Wrap(err, "config", "unable to package configuration files", nil)
	}
	b.configFiles = configFiles

	return nil
}

// computeConfigForService takes a given service and computes the configuration needed for it
func (b *builder) computeConfigForService(service *est.Service) error {
	cfg, err := cueutil.LoadFromFS(b.configFiles, service.Root.RelPath)
	if err != nil {
		return err
	}

	bytes, err := cfg.MarshalJSON()
	if err != nil {
		return eerror.Wrap(err, "config", "unable to marshal config to JSON", map[string]any{"service": service.Name})
	}
	b.configs[service.Name] = string(bytes)

	return nil
}
