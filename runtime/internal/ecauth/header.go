package ecauth

import (
	"crypto/hmac"
	"net/http"
	"strconv"
	"strings"
	"time"

	"encore.dev/beta/errs"
)

// Headers are the headers that are used to authenticate a request.
type Headers struct {
	Authorization string `header:"Authorization" encore:"sensitive"`
	Date          string `header:"Date"`
}

// Equal returns true if the headers are equal.
//
// It compares the Authorization and Date headers using
// hmac.Equal to prevent timing attacks.
func (h *Headers) Equal(other *Headers) bool {
	authMatches := hmac.Equal([]byte(h.Authorization), []byte(other.Authorization))
	dateMatches := hmac.Equal([]byte(h.Date), []byte(other.Date))
	return authMatches && dateMatches
}

// SigningComponents returns the components of the authorization header.
func (h *Headers) SigningComponents() (keyID uint32, appSlug, envName string, timestamp time.Time, operationHash OperationHash, err error) {
	switch {
	case h.Authorization == "":
		err = errs.B().Code(errs.InvalidArgument).Msg("Authorization header is required").Err()
		return
	case h.Date == "":
		err = errs.B().Code(errs.InvalidArgument).Msg("Date header is required").Err()
		return
	}

	// First parse the date header
	timestamp, err = http.ParseTime(h.Date)
	if err != nil {
		err = errs.B().Code(errs.InvalidArgument).Msg("invalid Date header").Err()
		return
	}

	scheme, parametersStr, found := strings.Cut(h.Authorization, " ")
	if !found {
		err = errs.B().Code(errs.InvalidArgument).Msg("invalid Authorization header: unable to find scheme").Err()
		return
	} else if scheme != authScheme {
		err = errs.B().Code(errs.InvalidArgument).Msg("unknown scheme in Authorization header").Err()
		return
	}

	// Extract the parameters parts
	parameters := strings.Split(parametersStr, ", ")
	if len(parameters) != 3 {
		err = errs.B().Code(errs.InvalidArgument).Msg("invalid Authorization header: expected 3 parameters").Err()
		return
	}

	for _, parameter := range parameters {
		name, value, found := strings.Cut(parameter, "=")
		if !found {
			err = errs.B().Code(errs.InvalidArgument).Msg("invalid Authorization header: unable to find parameter name").Err()
			return
		}

		switch name {
		case "cred":
			// Unquote the value
			value, err = strconv.Unquote(value)
			if err != nil {
				err = errs.B().Code(errs.InvalidArgument).Msg("invalid Authorization header: unable to unquote credential string").Err()
				return
			}

			var date string
			keyID, appSlug, envName, date, err = parseCredentialString(value)
			if err != nil {
				return
			}

			// Verify the date matches the date header
			if date != timestamp.UTC().Format("20060102") {
				err = errs.B().Code(errs.InvalidArgument).Msg("invalid Authorization header: dates don't align").Err()
				return
			}

		case "op":
			operationHash = OperationHash(value)

		case "sig":
		// No need to do anything with the signature

		default:
			err = errs.B().Code(errs.InvalidArgument).Msg("invalid Authorization header: unknown parameter").Err()
			return
		}
	}

	return
}

// parseCredentialString parses the credential string from the authorization header and extracts the
// key ID, app slug, environment name, and date.
func parseCredentialString(str string) (keyID uint32, appSlug, envName string, date string, err error) {
	parts := strings.Split(str, "/")
	if len(parts) != 4 {
		err = errs.B().Code(errs.InvalidArgument).Msg("invalid Authorization header: invalid credential string").Err()
		return
	}

	date = parts[0]
	appSlug = parts[1]
	envName = parts[2]

	keyID64, err := strconv.ParseUint(parts[3], 10, 32)
	if err != nil {
		err = errs.B().Code(errs.InvalidArgument).Cause(err).Msg("invalid Authorization header: invalid credential string").Err()
		return
	}
	keyID = uint32(keyID64)

	return
}
