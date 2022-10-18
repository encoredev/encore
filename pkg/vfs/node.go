package vfs

import (
	"time"
)

type node struct {
	name    string // The name of this node
	parent  *directoryContents
	modTime time.Time
}
