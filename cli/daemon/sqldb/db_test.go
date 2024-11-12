package sqldb

import (
	"io/fs"
	"testing"

	qt "github.com/frankban/quicktest"
	_ "github.com/golang-migrate/migrate/v4/source/file" // for running migrations from the filesystem
)

func TestFindClosestVersion(t *testing.T) {
	c := qt.New(t)
	testCases := map[string]struct {
		versions    []uint
		dirty       int
		expected    int
		expectedErr bool
	}{
		"first": {
			versions: []uint{1, 2, 3},
			dirty:    1,
			expected: -1,
		},
		"middle": {
			versions: []uint{1, 2, 3},
			dirty:    2,
			expected: 1,
		},
		"last": {
			versions: []uint{1, 2, 3},
			dirty:    3,
			expected: 2,
		},
		"deleted": {
			versions: []uint{1, 2, 4},
			dirty:    3,
			expected: 2,
		},
		"deleted_first": {
			versions: []uint{2, 3, 4},
			dirty:    1,
			expected: -1,
		},
		"empty": {
			dirty:       5,
			expectedErr: true,
		},
	}

	for name, tc := range testCases {
		c.Run(name, func(c *qt.C) {
			result, err := findClosestLowerVersion(func() (uint, error) {
				if len(tc.versions) == 0 {
					return 0, fs.ErrNotExist
				}
				return tc.versions[0], nil
			}, tc.dirty, func(version uint) (uint, error) {
				for _, v := range tc.versions {
					if v > version {
						return v, nil
					}
				}
				return 0, fs.ErrNotExist
			})
			if tc.expectedErr {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(result, qt.Equals, tc.expected)
			}
		})
	}
}
