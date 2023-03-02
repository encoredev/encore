package sym

import (
	"debug/macho"
	"fmt"
	"io"

	"encr.dev/v2/cli/internal/gosym"
)

func load(r io.ReaderAt) (*Table, error) {
	exe, err := macho.NewFile(r)
	if err != nil {
		return nil, err
	}
	defer exe.Close()

	text := exe.Section("__text")
	if text == nil {
		return nil, fmt.Errorf("cannot find __text section")
	}
	textAddr := text.Addr

	pctbl := exe.Section("__gopclntab")
	if pctbl == nil {
		return nil, fmt.Errorf("cannot find __gopclntab section")
	}
	pctblData, err := pctbl.Data()
	if err != nil {
		return nil, fmt.Errorf("cannot read __gopclntab: %v", err)
	}

	symtab := exe.Section("__gosymtab")
	if symtab == nil {
		return nil, fmt.Errorf("cannot find __gosymtab section")
	}
	symtabData, err := symtab.Data()
	if err != nil {
		return nil, fmt.Errorf("cannot read __gosymtab: %v", err)
	}

	lntbl := gosym.NewLineTable(pctblData, textAddr)
	tbl, err := gosym.NewTable(symtabData, lntbl)
	if err != nil {
		return nil, err
	}
	return &Table{Table: tbl, BaseOffset: textAddr}, nil
}
