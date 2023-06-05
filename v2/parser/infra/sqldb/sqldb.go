package sqldb

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/resourceparser"
)

type Database struct {
	AST          *ast.CallExpr
	Pkg          *pkginfo.Package
	Name         string // The database name
	Doc          string
	File         option.Option[*pkginfo.File]
	MigrationDir paths.MainModuleRelSlash
	Migrations   []MigrationFile
}

func (d *Database) Kind() resource.Kind       { return resource.SQLDatabase }
func (d *Database) Package() *pkginfo.Package { return d.Pkg }
func (d *Database) ResourceName() string      { return d.Name }
func (d *Database) Pos() token.Pos            { return token.NoPos }
func (d *Database) End() token.Pos            { return token.NoPos }
func (d *Database) SortKey() string           { return d.Name }

type MigrationFile struct {
	Filename    string
	Number      uint64
	Description string
}

var DatabaseParser = &resourceparser.Parser{
	Name: "SQL Database",

	InterestingImports: []paths.Pkg{"encore.dev/storage/sqldb"},
	Run: func(p *resourceparser.Pass) {
		name := pkginfo.QualifiedName{PkgPath: "encore.dev/storage/sqldb", Name: "NewDatabase"}

		spec := &parseutil.ReferenceSpec{
			MinTypeArgs: 0,
			MaxTypeArgs: 0,
			Parse:       parseDatabase,
		}

		parseutil.FindPkgNameRefs(p.Pkg, []pkginfo.QualifiedName{name}, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			parseutil.ParseReference(p, spec, parseutil.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
		})
	},
}

func parseDatabase(d parseutil.ReferenceInfo) {
	errs := d.Pass.Errs

	if len(d.Call.Args) != 2 {
		errs.Add(errNewDatabaseArgCount(len(d.Call.Args)).AtGoNode(d.Call))
		return
	}

	databaseName := parseutil.ParseResourceName(d.Pass.Errs, "sqldb.NewDatabase", "database name",
		d.Call.Args[0], parseutil.KebabName, "")
	if databaseName == "" {
		// we already reported the error inside ParseResourceName
		return
	}

	cfgLit, ok := literals.ParseStruct(d.Pass.Errs, d.File, "sqldb.DatabaseConfig", d.Call.Args[1])
	if !ok {
		return // error reported by ParseStruct
	}

	// Decode the config
	type decodedConfig struct {
		Migrations string `literal:",required"`
	}
	config := literals.Decode[decodedConfig](d.Pass.Errs, cfgLit, nil)

	if path.IsAbs(config.Migrations) {
		errs.Add(errNewDatabaseAbsPath.AtGoNode(cfgLit.Expr("Migrations")))
		return
	}
	migDir := filepath.FromSlash(config.Migrations)
	if !filepath.IsLocal(migDir) {
		errs.Add(errNewDatabaseNonLocalPath.AtGoNode(cfgLit.Expr("Migrations")))
		return
	}

	migrationDir := d.Pass.Pkg.FSPath.Join(migDir)
	if fi, err := os.Stat(migrationDir.ToIO()); os.IsNotExist(err) || (err == nil && !fi.IsDir()) {
		errs.Add(errNewDatabaseMigrationDirNotFound.AtGoNode(cfgLit.Expr("Migrations")))
		return
	} else if err != nil {
		errs.AddStd(err)
		return
	}

	// Compute the relative path to the migration directory from the main module.
	relMigrationDir, err := filepath.Rel(d.Pass.MainModuleDir.ToIO(), migrationDir.ToIO())
	if err != nil || !filepath.IsLocal(relMigrationDir) {
		errs.Add(errMigrationsNotInMainModule)
		return
	}

	migrations, err := parseMigrations(d.Pass.Pkg, migrationDir)
	if err != nil {
		errs.Add(errUnableToParseMigrations.Wrapping(err))
		return
	}

	db := &Database{
		AST:          d.Call,
		Pkg:          d.Pass.Pkg,
		Name:         databaseName,
		Doc:          d.Doc,
		MigrationDir: paths.MainModuleRelSlash(filepath.ToSlash(relMigrationDir)),
		Migrations:   migrations,
	}
	d.Pass.RegisterResource(db)
	d.Pass.AddBind(d.File, d.Ident, db)
}

var MigrationParser = &resourceparser.Parser{
	Name: "SQL Database",

	InterestingSubdirs: []string{"migrations"},
	Run: func(p *resourceparser.Pass) {
		migrationDir := p.Pkg.FSPath.Join("migrations")
		migrations, err := parseMigrations(p.Pkg, migrationDir)
		if err != nil {
			// HACK(andre): We should only look for migration directories inside services,
			// but when this code runs we don't yet know what services exist.
			// For now, use some heuristics to guess if this is a service and otherwise ignore it.
			if !pkgIsLikelyService(p.Pkg) {
				return
			}

			p.Errs.Add(errUnableToParseMigrations.Wrapping(err))
			return
		} else if len(migrations) == 0 {
			return
		}

		// HACK(andre): We also need to do the check here, otherwise we get
		// spurious databases that are defined outside of services.
		if !pkgIsLikelyService(p.Pkg) {
			return
		}

		// Compute the relative path to the migration directory from the main module.
		relMigrationDir, err := filepath.Rel(p.MainModuleDir.ToIO(), migrationDir.ToIO())
		if err != nil || !filepath.IsLocal(relMigrationDir) {
			p.Errs.Add(errMigrationsNotInMainModule)
			return
		}

		res := &Database{
			Pkg:          p.Pkg,
			Name:         p.Pkg.Name,
			MigrationDir: paths.MainModuleRelSlash(filepath.ToSlash(relMigrationDir)),
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
		num, err := strconv.ParseUint(match[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("migration %s/migrations/%s has an invalid version number %q (must be a positive integer)",
				pkg.Name, f.Name(), match[1])
		}
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
	return migrations, nil
}

func pkgIsLikelyService(pkg *pkginfo.Package) bool {
	isLikelyService := func(file *pkginfo.File) bool {
		contents := file.Contents()
		switch {
		case bytes.Contains(contents, []byte("encore:api")):
			return true
		case bytes.Contains(contents, []byte("pubsub.NewSubscription")):
			return true
		case bytes.Contains(contents, []byte("encore:authhandler")):
			return true
		case bytes.Contains(contents, []byte("encore:service")):
			return true
		default:
			return false
		}
	}

	for _, file := range pkg.Files {
		if isLikelyService(file) {
			return true
		}
	}
	return false
}
