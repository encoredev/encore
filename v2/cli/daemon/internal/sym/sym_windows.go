package sym

import (
	"debug/pe"
	"fmt"
	"io"

	"encr.dev/v2/cli/internal/gosym"
)

// This code is a simplified version of $GOROOT/src/cmd/internal/objfile/pe.go.

func load(r io.ReaderAt) (*Table, error) {
	exe, err := pe.NewFile(r)
	if err != nil {
		return nil, err
	}
	defer exe.Close()

	var imageBase, textStart uint64
	switch oh := exe.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		imageBase = uint64(oh.ImageBase)
	case *pe.OptionalHeader64:
		imageBase = oh.ImageBase
	default:
		return nil, fmt.Errorf("pe file format not recognized")
	}
	if sect := exe.Section(".text"); sect != nil {
		textStart = imageBase + uint64(sect.VirtualAddress)
	}
	pclntab, err := loadPETable(exe, "runtime.pclntab", "runtime.epclntab")
	if err != nil {
		return nil, err
	}
	symtab, err := loadPETable(exe, "runtime.symtab", "runtime.esymtab")
	if err != nil {
		return nil, err
	}

	lntbl := gosym.NewLineTable(pclntab, textStart)
	tbl, err := gosym.NewTable(symtab, lntbl)
	if err != nil {
		return nil, err
	}
	return &Table{Table: tbl, BaseOffset: textStart}, nil
}

func findPESymbol(f *pe.File, name string) (*pe.Symbol, error) {
	for _, s := range f.Symbols {
		if s.Name != name {
			continue
		}
		if s.SectionNumber <= 0 {
			return nil, fmt.Errorf("symbol %s: invalid section number %d", name, s.SectionNumber)
		}
		if len(f.Sections) < int(s.SectionNumber) {
			return nil, fmt.Errorf("symbol %s: section number %d is larger than max %d", name, s.SectionNumber, len(f.Sections))
		}
		return s, nil
	}
	return nil, fmt.Errorf("no %s symbol found", name)
}

func loadPETable(f *pe.File, sname, ename string) ([]byte, error) {
	ssym, err := findPESymbol(f, sname)
	if err != nil {
		return nil, err
	}
	esym, err := findPESymbol(f, ename)
	if err != nil {
		return nil, err
	}
	if ssym.SectionNumber != esym.SectionNumber {
		return nil, fmt.Errorf("%s and %s symbols must be in the same section", sname, ename)
	}
	sect := f.Sections[ssym.SectionNumber-1]
	data, err := sect.Data()
	if err != nil {
		return nil, err
	}
	return data[ssym.Value:esym.Value], nil
}
