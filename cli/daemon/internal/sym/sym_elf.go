//go:build !windows && !darwin
// +build !windows,!darwin

package sym

import (
	"debug/elf"
	"fmt"
	"io"

	"encr.dev/cli/internal/gosym"
)

func load(r io.ReaderAt) (*Table, error) {
	exe, err := elf.NewFile(r)
	if err != nil {
		return nil, err
	}
	defer exe.Close()

	text := exe.Section(".text")
	if text == nil {
		return nil, fmt.Errorf("cannot find .text section")
	}
	textAddr := text.Addr

	pctbl := exe.Section(".gopclntab")
	if pctbl == nil {
		return nil, fmt.Errorf("cannot find .gopclntab section")
	}
	pctblData, err := pctbl.Data()
	if err != nil {
		return nil, fmt.Errorf("cannot read .gopclntab: %v", err)
	}

	symtab := exe.Section(".gosymtab")
	if symtab == nil {
		return nil, fmt.Errorf("cannot find .gosymtab section")
	}
	symtabData, err := symtab.Data()
	if err != nil {
		return nil, fmt.Errorf("cannot read .gosymtab: %v", err)
	}

	lntbl := gosym.NewLineTable(pctblData, textAddr)
	tbl, err := gosym.NewTable(symtabData, lntbl)
	if err != nil {
		return nil, err
	}
	return &Table{Table: tbl, BaseOffset: textAddr}, nil
}
