// Copyright (C) 2013-2018 by Maxim Bublis <b@codemonkey.ru>
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to
// the following conditions:
//
// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
// LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
// OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
// WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package uuid

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"hash"
	"io"
	"os"
	"sync"
	"time"
)

// Difference in 100-nanosecond intervals between
// UUID epoch (October 15, 1582) and Unix epoch (January 1, 1970).
const epochStart = 122192928000000000

type epochFunc func() time.Time

var (
	posixUID = uint32(os.Getuid())
	posixGID = uint32(os.Getgid())
)

// NewV3 returns a UUID based on the MD5 hash of the namespace UUID and name.
func NewV3(ns UUID, name string) UUID {
	return g.NewV3(ns, name)
}

// NewV4 returns a randomly generated UUID.
func NewV4() (UUID, error) {
	return g.NewV4()
}

// NewV5 returns a UUID based on SHA-1 hash of the namespace UUID and name.
func NewV5(ns UUID, name string) UUID {
	return g.NewV5(ns, name)
}

// gen is a reference UUID generator based on the specifications laid out in
// RFC-4122 and DCE 1.1: Authentication and Security Services. This type
// satisfies the Generator interface as defined in this package.
//
// For consumers who are generating V1 UUIDs, but don't want to expose the MAC
// address of the node generating the UUIDs, the NewGenWithHWAF() function has been
// provided as a convenience. See the function's documentation for more info.
//
// The authors of this package do not feel that the majority of users will need
// to obfuscate their MAC address, and so we recommend using NewGen() to create
// a new generator.
type gen struct {
	clockSequenceOnce sync.Once
	storageMutex      sync.Mutex

	rand io.Reader

	epochFunc     epochFunc
	lastTime      uint64
	clockSequence uint16
}

// newGen returns a new instance of gen with some default values set.
var g = &gen{
	epochFunc: time.Now,
	rand:      rand.Reader,
}

// NewV3 returns a UUID based on the MD5 hash of the namespace UUID and name.
func (g *gen) NewV3(ns UUID, name string) UUID {
	u := newFromHash(md5.New(), ns, name)
	u.SetVersion(V3)
	u.setVariant(variantRFC4122)

	return u
}

// NewV4 returns a randomly generated UUID.
func (g *gen) NewV4() (UUID, error) {
	u := UUID{}
	if _, err := io.ReadFull(g.rand, u[:]); err != nil {
		return Nil, err
	}
	u.SetVersion(V4)
	u.setVariant(variantRFC4122)

	return u, nil
}

// NewV5 returns a UUID based on SHA-1 hash of the namespace UUID and name.
func (g *gen) NewV5(ns UUID, name string) UUID {
	u := newFromHash(sha1.New(), ns, name)
	u.SetVersion(V5)
	u.setVariant(variantRFC4122)

	return u
}

// Returns the epoch and clock sequence.
func (g *gen) getClockSequence() (uint64, uint16, error) {
	var err error
	g.clockSequenceOnce.Do(func() {
		buf := make([]byte, 2)
		if _, err = io.ReadFull(g.rand, buf); err != nil {
			return
		}
		g.clockSequence = binary.BigEndian.Uint16(buf)
	})
	if err != nil {
		return 0, 0, err
	}

	g.storageMutex.Lock()
	defer g.storageMutex.Unlock()

	timeNow := g.getEpoch()
	// Clock didn't change since last UUID generation.
	// Should increase clock sequence.
	if timeNow <= g.lastTime {
		g.clockSequence++
	}
	g.lastTime = timeNow

	return timeNow, g.clockSequence, nil
}

// Returns the difference between UUID epoch (October 15, 1582)
// and current time in 100-nanosecond intervals.
func (g *gen) getEpoch() uint64 {
	return epochStart + uint64(g.epochFunc().UnixNano()/100)
}

// Returns the UUID based on the hashing of the namespace UUID and name.
func newFromHash(h hash.Hash, ns UUID, name string) UUID {
	u := UUID{}
	h.Write(ns[:])
	h.Write([]byte(name))
	copy(u[:], h.Sum(nil))

	return u
}
