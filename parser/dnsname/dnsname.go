package dnsname

import (
	"regexp"
)

const dns1035LabelFmt string = "[a-z]([-a-z0-9]*[a-z0-9])?"

// DNS1035LabelMaxLength is a label's max length in DNS (RFC 1035)
const DNS1035LabelMaxLength int = 63

var Dns1035LabelRegexp = regexp.MustCompile("^" + dns1035LabelFmt + "$")
