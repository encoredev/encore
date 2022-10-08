package srcerrors

import (
	"strings"
)

func combine(parts ...string) string {
	return strings.Join(parts, "\n\n")
}

const (
	internalErrReportToEncore = "This is a bug in Encore and should not have occurred. Please report this issue to the " +
		"Encore team either on Github at https://github.com/encoredev/encore/issues/new and include this error."

	makeService = "To make this package a count as a service, this package or one of it's parents must have either one " +
		"or more API's declared within them or a PubSub subscription."

	configHelp = "For more information on configuration, see https://encore.dev/docs/develop/config"
)
