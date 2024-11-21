package gcsutil

//go:generate protoc --go_out=. --go_opt=paths=source_relative gcspagetoken.proto

import (
	"encoding/base64"
	"fmt"

	"google.golang.org/protobuf/proto"
)

// EncodePageToken returns a synthetic page token to find files greater than the given string.
// If this is part of a prefix query, the token should fall within the prefixed range.
// BRITTLE: relies on a reverse-engineered internal GCS token format, which may be subject to change.
func EncodePageToken(greaterThan string) string {
	bytes, err := proto.Marshal(&GcsPageToken{
		LastFile: greaterThan,
	})
	if err != nil {
		panic("could not encode gcsPageToken:" + err.Error())
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

// DecodePageToken decodes a GCS pageToken to the name of the last file returned.
func DecodePageToken(pageToken string) (string, error) {
	bytes, err := base64.StdEncoding.DecodeString(pageToken)
	if err != nil {
		return "", fmt.Errorf("could not base64 decode pageToken %s: %w", pageToken, err)
	}
	var message GcsPageToken
	if err := proto.Unmarshal(bytes, &message); err != nil {
		return "", fmt.Errorf("could not unmarshal proto: %w", err)
	}

	return message.LastFile, nil
}
