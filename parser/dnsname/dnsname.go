package dnsname

import (
	"errors"
	"fmt"
	"regexp"
)

const dns1035LabelFmt string = "[a-z]([-a-z0-9]*[a-z0-9])?"
const dns1035LabelErrMsg string = "a DNS-1035 label must consist of lower case alphanumeric characters or '-', start with an alphabetic character, and end with an alphanumeric character"

// DNS1035LabelMaxLength is a label's max length in DNS (RFC 1035)
const DNS1035LabelMaxLength int = 63

var dns1035LabelRegexp = regexp.MustCompile("^" + dns1035LabelFmt + "$")

// DNS1035Label tests for a string that conforms to the definition of a label in
// DNS (RFC 1035).
// A DNS-1035 label must consist of lower case alphanumeric characters or '-',
// start with an alphabetic character, and end with an alphanumeric character.
func DNS1035Label(value string) error {
	if len(value) > DNS1035LabelMaxLength {
		return maxLenError(DNS1035LabelMaxLength)
	}
	if !dns1035LabelRegexp.MatchString(value) {
		return regexError(dns1035LabelErrMsg, dns1035LabelFmt, "my-name", "abc-123")
	}
	return nil
}

// MaxLenError returns a string explanation of a "string too long" validation
// failure.
func maxLenError(length int) error {
	return fmt.Errorf("must be no more than %d characters", length)
}

// RegexError returns a string explanation of a regex validation failure.
func regexError(msg string, fmt string, examples ...string) error {
	if len(examples) == 0 {
		return errors.New(msg + " (regex used for validation is '" + fmt + "')")
	}
	msg += " (e.g. "
	for i := range examples {
		if i > 0 {
			msg += " or "
		}
		msg += "'" + examples[i] + "', "
	}
	msg += "regex used for validation is '" + fmt + "')"
	return errors.New(msg)
}
