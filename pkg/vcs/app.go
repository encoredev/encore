package vcs

import (
	"github.com/rs/zerolog/log"
)

// GetRevision returns the version control system (VCS) revision information for the Encore application.
//
// If there is an error getting the revision information, no revision information is returned and the App is flagged as
// having uncommitted files. This will happen most likely because no supported VCS system can be found.
//
// Supported VCS systems include;
//  - Hg
//  - Git
//  - Svn
//  - Bzr
//  - Fossil
func GetRevision(appRoot string) Status {
	appRoot, cmd, err := FromDir(appRoot, "", false)
	if err != nil {
		log.Err(err).Str("app", appRoot).Msg("unable to determine VCS system")
		return Status{Uncommitted: true}
	}

	status, err := cmd.Status(cmd, appRoot)
	if err != nil {
		log.Err(err).Str("app", appRoot).Msg("unable to get VCS status")
		return Status{Uncommitted: true}
	}

	return status
}
