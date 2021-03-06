// Package elf converts ELF programs to LE/LX programs.
package elf

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"moria.us/elf2dos/module"
)

// objAbsolute is a placeholder object for absolute symbols.
const objAbsolute int32 = int32(^uint32(0) >> 1)

// A wrappedError is an error wrapped with a location for context.
type wrappedError struct {
	location string
	inner    error
}

func (e *wrappedError) Error() string {
	return fmt.Sprintf("%s: %v", e.location, e.inner)
}

// wrapError returns an error wrapped with a location for context.
func wrapError(e error, loc string) error {
	if we, ok := e.(*wrappedError); ok {
		return &wrappedError{
			location: loc + ": " + we.location,
			inner:    we.inner,
		}
	}
	return &wrappedError{
		location: loc,
		inner:    e,
	}
}

// wrapError returns an error wrapped with a location for context.
func wrapErrorf(e error, f string, a ...interface{}) error {
	return wrapError(e, fmt.Sprintf(f, a...))
}

func wrapErrorSection(e error, i int, s *elf.Section) error {
	return wrapErrorf(e, "section %d %q", i, s.Name)
}

func wrapErrorSegment(e error, i int) error {
	return wrapErrorf(e, "segment %d", i)
}

// =================================================================================================

// ptGNUEHFrame is an ELF segment type containing exception handling
// information.
const ptGNUEHFrame elf.ProgType = 0x6474e551

// An addrRange is a range of addresses in the ELF file.
type addrRange struct {
	addr uint32
	size uint32
}

// hasAddr returns true if the range contains the given address, or if the
// address is one past the end of the range.
func (x addrRange) hasAddr(addr uint32) bool {
	return x.addr <= addr && addr <= x.addr+x.size
}

// overlaps returns true if the ranges contain any bytes in common.
func (x addrRange) overlaps(y addrRange) bool {
	return x.addr+x.size > y.addr && y.addr+y.size > x.addr
}

// overlaps returns true if x contains all of y.
func (x addrRange) contains(y addrRange) bool {
	return x.addr <= y.addr && y.addr+y.size <= x.addr+x.size
}

// A segment is an assignment of an ELF segment to an LE/LX object.
type segment struct {
	addrRange
	index  int
	prog   *elf.Prog
	object *module.Object
}

// resolveAddr resolves an ELF address as an LE/LX object reference.
func resolveAddr(segs []segment, addr uint32) (r module.Ref) {
	for i, s := range segs {
		if s.hasAddr(addr) {
			r.Obj = int32(i + 1)
			r.Off = int32(addr - s.addr)
			break
		}
	}
	return
}

// A symbol is the resolution of an ELF symbol to an LE/LX reference.
type symbol struct {
	addr uint32
	module.Ref
	name string
}

// readLoadSegment reads a PT_LOAD segment and returns the assigned LE/LX
// object.
func readLoadSegment(i int, p *elf.Prog) (seg segment, err error) {
	flags := module.Obj32Bit
	if p.Flags&elf.PF_X != 0 {
		flags |= module.ObjX
	}
	if p.Flags&elf.PF_W != 0 {
		flags |= module.ObjW
	}
	if p.Flags&elf.PF_R != 0 {
		flags |= module.ObjR
	} else {
		return segment{}, errors.New("segment is loadable but not readable, which is unsupported")
	}
	const knownFlags = elf.PF_X | elf.PF_W | elf.PF_R
	if unknownFlags := p.Flags &^ knownFlags; unknownFlags != 0 {
		return segment{}, fmt.Errorf("segment has unknown flags 0x%08x", uint32(unknownFlags))
	}
	addr := uint32(p.Vaddr)
	size := uint32(p.Memsz)
	var data []byte
	if dsz := p.Filesz; dsz > 0 {
		data = make([]byte, dsz)
		if _, err := p.ReadAt(data, 0); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return segment{}, fmt.Errorf("could not read segment: %v", err)
		}
	}
	return segment{
		addrRange: addrRange{
			addr: addr,
			size: size,
		},
		index: i,
		prog:  p,
		object: &module.Object{
			ObjectHeader: module.ObjectHeader{
				VirtualSize: size,
				BaseAddress: addr,
				Flags:       flags,
			},
			Data: data,
		},
	}, nil
}

// assignSegments assigns each segment in an ELF file to an LE/LX object.
func assignSegments(f *elf.File) ([]segment, error) {
	var segments []segment
	for i, p := range f.Progs {
		switch p.Type {
		case elf.PT_NULL, elf.PT_NOTE, ptGNUEHFrame:
			// NULL means discard, we don't want to keep comments, and we
			// explicitly discard exception handling information.
		case elf.PT_LOAD:
			seg, err := readLoadSegment(i, p)
			if err != nil {
				return nil, wrapErrorSegment(err, i)
			}
			segments = append(segments, seg)
		default:
			return nil, wrapErrorSegment(
				fmt.Errorf("segment has type %s, which is unsupported", p.Type), i)
		}
	}
	return segments, nil
}

// resolveSymbols resolves each symbol in an ELF file to an LE/LX object
// reference.
func resolveSymbols(f *elf.File, segs []segment) ([]symbol, error) {
	// Map sections to objects.
	secObjects := make([]int, len(f.Sections))
	for i, s := range f.Sections {
		offset := uint32(s.Addr)
		obj := -1
		for _, seg := range segs {
			if seg.addr <= offset && offset < seg.addr+seg.size {
				obj = seg.index
				break
			}
		}
		secObjects[i] = obj
	}
	syms, err := f.Symbols()
	if err != nil {
		return nil, err
	}
	osyms := make([]symbol, len(syms))
	for i, sym := range syms {
		osyms[i].addr = uint32(sym.Value)
		osyms[i].name = sym.Name
		// Find the object using the symbol's section.
		if 0 <= sym.Section && int(sym.Section) < len(secObjects) {
			obj := secObjects[sym.Section]
			seg := segs[obj]
			osyms[i].Ref = module.Ref{
				Obj: int32(obj + 1),
				Off: int32(uint32(sym.Value) - seg.addr),
			}
		} else if sym.Section == elf.SHN_ABS {
			osyms[i].Ref.Obj = objAbsolute
		} else {
			return nil, fmt.Errorf("symbol has invalid section %d", sym.Section)
		}
	}
	return osyms, nil
}

func addRelocation(rel elf.Rel32, segs []segment, syms []symbol) error {
	// Find segment containing the relocation source (where the fixup applies).
	var seg segment
	var srcObj int32
	for i, s := range segs {
		if s.contains(addrRange{rel.Off, 4}) {
			seg = s
			srcObj = int32(i + 1)
			break
		}
	}
	if srcObj == 0 {
		// The relocation does not exist in any segment, which may mean that we
		// have discarded the segment containing it. This can happen to EH frame
		// data.
		return nil
	}
	// Get the relocation target, which is a symbol.
	rsym := rel.Info >> 8
	if rsym == 0 || rsym > uint32(len(syms)) {
		return fmt.Errorf("symbol reference %d out of bounds", rsym)
	}
	sym := syms[rsym-1]
	if sym.Obj == 0 {
		return fmt.Errorf("unresolved symbol %q (symbol %d)", sym.name, rsym)
	}
	if sym.Obj == objAbsolute {
		return nil
	}
	// Get the current value stored in the relocation. Note that the value here
	// is after the relocations are applied by the ELF linker.
	obj := seg.object
	srcOff := int32(rel.Off - seg.addr)
	val := binary.LittleEndian.Uint32(obj.Data[srcOff:])
	var srcType module.SrcType
	var fixOff int32
	switch rtype := elf.R_386(rel.Info & 0xff); rtype {
	case elf.R_386_32:
		srcType = module.SrcOffset32
		fixOff = sym.Off + int32(val-sym.addr)
	case elf.R_386_PC32:
		if sym.Obj == srcObj {
			// Note that: srcOff+int32(val)+4 == fixOff
			// Relative fixups within an object are not necessary.
			return nil
		}
		srcType = module.SrcRelative32
		fixOff = sym.Off + int32(val+rel.Off+4-sym.addr)
	default:
		return fmt.Errorf("unsupported relocation type %s", rtype)
	}
	obj.Fixups = append(obj.Fixups, module.Fixup{
		SrcType: srcType,
		Src:     srcOff,
		Target: module.Ref{
			Obj: sym.Obj,
			Off: fixOff,
		},
	})
	return nil
}

// readRelocationSection reads a single relocation section and adds its fixups
// to the objects.
func readRelocationSection(s *elf.Section, segs []segment, syms []symbol) error {
	data, err := s.Data()
	if err != nil {
		return err
	}
	r := bytes.NewReader(data)
	switch s.Type {
	case elf.SHT_REL:
		if len(data)&7 != 0 {
			return errors.New("REL section length is not a multiple of 8")
		}
		for r.Len() > 0 {
			var rel elf.Rel32
			binary.Read(r, binary.LittleEndian, &rel)
			if err := addRelocation(rel, segs, syms); err != nil {
				return wrapErrorf(err, "relocation at 0x%x", rel.Off)
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported relocation section type %s", s.Type)
	}
}

// readSections reads the sections in an ELF file and applies all relevant
// changes to the segments.
func readSections(f *elf.File, segs []segment, syms []symbol) error {
	for i, s := range f.Sections {
		switch s.Type {
		case elf.SHT_REL, elf.SHT_RELA:
			bi := int(s.Info)
			if bi < 0 || len(f.Sections) <= bi {
				return wrapErrorSection(
					errors.New("relocation section refers to invalid section"), i, s)
			}
			if err := readRelocationSection(s, segs, syms); err != nil {
				return wrapErrorSection(err, i, s)
			}
		}
	}
	return nil
}

// ConvertToLELX reads an ELF executable and returns an LE/LX program.
func ConvertToLELX(name string) (*module.Program, error) {
	f, err := elf.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if f.Class != elf.ELFCLASS32 {
		return nil, fmt.Errorf("ELF has class %s, expected ELFCLASS32", f.Class)
	}
	if f.Data != elf.ELFDATA2LSB {
		return nil, fmt.Errorf("ELF has data %s, expected ELFDATA2LSB", f.Data)
	}
	if f.Type != elf.ET_EXEC {
		return nil, fmt.Errorf("ELF has type %s, expected ET_EXEC", f.Type)
	}
	if f.Machine != elf.EM_386 {
		return nil, fmt.Errorf("ELF Has machine %s, expected EM_386", f.Machine)
	}
	segs, err := assignSegments(f)
	if err != nil {
		return nil, err
	}
	entry := resolveAddr(segs, uint32(f.Entry))
	if entry.Obj == 0 {
		return nil, fmt.Errorf("could not resolve entry point 0x%0x", f.Entry)
	}
	syms, err := resolveSymbols(f, segs)
	if err != nil {
		return nil, err
	}
	var stack module.Ref
	for _, sym := range syms {
		if sym.name == "_stack_end" {
			stack = sym.Ref
		}
	}
	if stack.Obj == 0 {
		return nil, errors.New("could not find _stack_end")
	}
	if err := readSections(f, segs, syms); err != nil {
		return nil, err
	}
	var objs []*module.Object
	for _, seg := range segs {
		objs = append(objs, seg.object)
	}
	return &module.Program{
		ProgramHeader: module.ProgramHeader{
			EIP: entry,
			ESP: stack,
		},
		Objects: objs,
	}, nil
}
