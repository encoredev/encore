package srcerrors

import (
	"fmt"
)

const internalErrReportToEncore = "This is a bug in Encore and should not have occurred. Please report this issue to the " +
	"Encore team either on Github at https://github.com/encoredev/encore/issues/new and include this error."

func resourceNameHelpKebabCase(resourceName string, paramName string) string {
	return fmt.Sprintf("%s %s's must be defined as string literals, "+
		"be between 1 and 63 characters long, and defined in \"kebab-case\", meaning it must start with a letter, end with a letter "+
		"or number and only contain lower case letters, numbers and dashes.",
		resourceName, paramName,
	)
}

func resourceNameHelpSnakeCase(resourceName string, paramName string) string {
	return fmt.Sprintf("%s %s's must be defined as string literals, "+
		"be between 1 and 63 characters long, and defined in \"snake_case\", meaning it must start with a letter, end with a letter "+
		"or number and only contain lower case letters, numbers and underscores.",
		resourceName, paramName,
	)
}
