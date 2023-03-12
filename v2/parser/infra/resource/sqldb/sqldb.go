package sqldb

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/infra/resource"
)

type Database struct {
	Pkg          *pkginfo.Package
	Name         string // The database name
	MigrationDir paths.FS
	Migrations   []MigrationFile
}

func (d *Database) Kind() resource.Kind       { return resource.SQLDatabase }
func (d *Database) Package() *pkginfo.Package { return d.Pkg }
func (d *Database) ResourceName() string      { return d.Name }

type MigrationFile struct {
	Filename    string
	Number      int
	Description string
}

var DatabaseParser = &resource.Parser{
	Name: "SQL Database",

	InterestingSubdirs: []string{"migrations"},
	Run: func(p *resource.Pass) {
		migrationDir := p.Pkg.FSPath.Join("migrations")
		migrations, err := parseMigrations(p.Pkg, migrationDir)
		if err != nil {
			p.Errs.Add(errUnableToParseMigrations.Wrapping(err))
			return
		} else if len(migrations) == 0 {
			return
		}

		res := &Database{
			Pkg:          p.Pkg,
			Name:         p.Pkg.Name,
			MigrationDir: migrationDir,
			Migrations:   migrations,
		}
		p.RegisterResource(res)
		p.AddImplicitBind(res)
	},
}

var migrationRe = regexp.MustCompile(`^(\d+)_([^.]+)\.(up|down).sql$`)

func parseMigrations(pkg *pkginfo.Package, migrationDir paths.FS) ([]MigrationFile, error) {
	files, err := os.ReadDir(migrationDir.ToIO())
	if err != nil {
		return nil, fmt.Errorf("could not read migrations: %v", err)
	}
	migrations := make([]MigrationFile, 0, len(files))
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		// If the file is not an SQL file ignore it, to allow for other files to be present
		// in the migration directory. For SQL files we want to ensure they're properly named
		// so that we complain loudly about potential typos. (It's theoretically possible to
		// typo the filename extension as well, but it's less likely due to syntax highlighting).
		if filepath.Ext(strings.ToLower(f.Name())) != ".sql" {
			continue
		}

		match := migrationRe.FindStringSubmatch(f.Name())
		if match == nil {
			return nil, fmt.Errorf("migration %s/migrations/%s has an invalid name (must be of the format '[123]_[description].[up|down].sql')",
				pkg.Name, f.Name())
		}
		num, _ := strconv.Atoi(match[1])
		if match[3] == "up" {
			migrations = append(migrations, MigrationFile{
				Filename:    f.Name(),
				Number:      num,
				Description: match[2],
			})
		}
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Number < migrations[j].Number
	})
	for i := 0; i < len(migrations); i++ {
		fn := migrations[i].Filename
		num := migrations[i].Number
		if num <= 0 {
			return nil, fmt.Errorf("%s/migrations/%s: invalid migration number %d", pkg.Name, fn, num)
		} else if num < (i + 1) {
			return nil, fmt.Errorf("%s/migrations/%s: duplicate migration with number %d", pkg.Name, fn, num)
		} else if num > (i + 1) {
			return nil, fmt.Errorf("%s/migrations/%s: missing migration with number %d", pkg.Name, fn, i+1)
		}
	}
	return migrations, nil
}
