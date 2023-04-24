// Package ecauth provides a way for Encore applications to authenticate against Encore cloud services.
//
// It provides a [Sign] function that can be used to sign a request and generate the required headers
// for including an API call to Encore platform services. The design of the Authorization header is based
// on AWS Signature Version 4, with the addition of a [OperationHash] value which includes a hash of the
// payload of the request.
package ecauth
