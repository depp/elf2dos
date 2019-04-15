package module

import (
	"bufio"
	"fmt"
	"strconv"
)

const indentLevel = "  "

const hexDigits = "0123456789abcdef"

func writeHexStr(w *bufio.Writer, b []byte) {
	d := make([]byte, 4*len(b)+3)
	j := 3*len(b) + 2
	for i, c := range b {
		d[i*3+0] = hexDigits[c>>4]
		d[i*3+1] = hexDigits[c&15]
		d[i*3+2] = ' '
		if 0x20 <= c && c <= 0x7e {
			d[j+i] = c
		}
	}
	d[j-2] = ' '
	d[j-1] = '"'
	d[4*len(b)+2] = '"'
	w.Write(d)
}

func endian(b byte) string {
	switch b {
	case 0:
		return "little endian"
	case 1:
		return "big endian"
	default:
		return "unknown"
	}
}

func cpuType(v uint16) string {
	switch v {
	case 1:
		return "80286"
	case 2:
		return "80386"
	case 3:
		return "80486"
	default:
		return "unknown"
	}
}

func osType(v uint16) string {
	switch v {
	case 1:
		return "OS/2"
	case 2:
		return "Windows"
	case 3:
		return "DOS 4.x"
	case 4:
		return "Windows 386"
	default:
		return "unknown"
	}
}

func writeInt0(w *bufio.Writer, v uint32, sz uint) {
	for i := uint(sz * 2); i > 0; i-- {
		w.WriteByte(hexDigits[(v>>((i-1)*4))&15])
	}
}

func writeInt(w *bufio.Writer, v uint32, sz uint) {
	w.WriteString("0x")
	writeInt0(w, v, sz)
}

type field struct {
	name string
	data interface{}
	hint string
}

func dumpFields(w *bufio.Writer, prefix string, fields []field) {
	if len(fields) == 0 {
		return
	}
	var (
		minName = int(^uint(0) >> 1)
		maxName int
	)
	for _, f := range fields {
		if len(f.name) > maxName {
			maxName = len(f.name)
		}
		if len(f.name) < minName {
			minName = len(f.name)
		}
	}
	spaces := make([]byte, maxName+2-minName)
	for i := range spaces {
		spaces[i] = ' '
	}
	for _, f := range fields {
		w.WriteString(prefix)
		w.WriteString(f.name)
		w.WriteByte(':')
		w.Write(spaces[:maxName+2-len(f.name)])
		switch v := f.data.(type) {
		case []byte:
			writeHexStr(w, v)
		case uint8:
			writeInt(w, uint32(v), 1)
		case uint16:
			writeInt(w, uint32(v), 2)
		case uint32:
			writeInt(w, v, 4)
		case Ref:
			writeInt(w, uint32(v.Obj), 4)
			w.WriteByte(':')
			writeInt(w, uint32(v.Off), 4)
		default:
			panic("unknown field type for " + f.name)
		}
		if f.hint != "" {
			w.WriteString("  ")
			w.WriteString(f.hint)
		}
		w.WriteByte('\n')
	}
}

// DumpText writes the object header, in text format, to the writer.
func (h *ObjectHeader) DumpText(w *bufio.Writer, prefix string) {
	dumpFields(w, prefix, []field{
		{"Virtual Size", h.VirtualSize, ""},
		{"Base Address", h.BaseAddress, ""},
		{"Flags", uint32(h.Flags), ""},
		{"Page Table Index", h.PageTableIndex, ""},
		{"Page Table Entries", h.NumPageTableEntries, ""},
		{"Reserved", h.Reserved, ""},
	})
}

func writeFixup(w *bufio.Writer, f Fixup) {
	writeInt0(w, uint32(f.SrcType), 1)
	w.WriteByte(':')
	if f.SrcType&0x20 != 0 {
		w.WriteByte('L')
	} else {
		w.WriteByte('-')
	}
	if f.SrcType&0x10 != 0 {
		w.WriteByte('A')
	} else {
		w.WriteByte('-')
	}
	var t string
	switch f.SrcType & 15 {
	case 0:
		t = "ab" // byte
	case 2:
		t = "sw" // selector word
	case 3:
		t = "fw" // far word
	case 5:
		t = "aw" // absolute word
	case 6:
		t = "fd" // far doubleword
	case 7:
		t = "ad" // absolute doubleword
	case 8:
		t = "rd" // relative doubleword
	default:
		t = "??"
	}
	w.WriteString(t)

	w.WriteByte(' ')
	if f.Src >= 0 {
		w.WriteByte('+')
		writeInt(w, uint32(f.Src), 2)
	} else {
		w.WriteByte('-')
		writeInt(w, uint32(-f.Src), 2)
	}

	w.WriteByte(' ')
	if f.Target.Obj > 0xff {
		writeInt0(w, uint32(f.Target.Obj), 2)
	} else {
		writeInt0(w, uint32(f.Target.Obj), 1)
	}
	w.WriteByte(':')
	if f.Target.Off > 0xffff {
		writeInt0(w, uint32(f.Target.Off), 4)
	} else {
		writeInt0(w, uint32(f.Target.Off), 2)
	}
}

// DumpText writes the object, in text format, to the writer
func (o *Object) DumpText(w *bufio.Writer, prefix string) {
	nprefix3 := prefix + indentLevel + indentLevel + indentLevel
	nprefix2 := nprefix3[:len(prefix)+len(indentLevel)*2]
	nprefix1 := nprefix3[:len(prefix)+len(indentLevel)]
	w.WriteString(prefix)
	w.WriteString("Header:\n")
	o.ObjectHeader.DumpText(w, nprefix1)
	if len(o.Pages) != 0 {
		w.WriteString(nprefix1)
		w.WriteString("Pages:\n")
		for i, p := range o.Pages {
			fmt.Fprintf(w, "%sPage %d, Fixup Page %d (Reserved: 0x%02x 0x%02x)\n",
				nprefix2, i, p.FixupPageIndex, p.Reserved1, p.Reserved2)
			for _, f := range p.Fixups {
				w.WriteString(nprefix3)
				writeFixup(w, f)
				w.WriteByte('\n')
			}
		}
	}
}

// DumpText writes the program header, in text format, to the writer.
func (p *ProgramHeader) DumpText(w *bufio.Writer, prefix string) {
	dumpFields(w, prefix, []field{
		{"Signature", p.Signature[:], ""},
		{"Byte Order", p.ByteOrder, endian(p.ByteOrder)},
		{"Word Order", p.WordOrder, endian(p.WordOrder)},
		{"Format Level", p.FormatLevel, ""},
		{"CPU Type", p.CPUType, cpuType(p.CPUType)},
		{"OS Type", p.OSType, osType(p.OSType)},
		{"Module Version", p.ModuleVersion, ""},
		{"Module Flags", p.ModuleFlags, ""},
		{"Module Num Pages", p.ModuleNumPages, ""},
		{"EIP", p.EIP, ""},
		{"ESP", p.ESP, ""},
		{"Page Size", p.PageSize, ""},
		{"Last Page Size", p.LastPageSize, ""},
		{"Fixup Section Size", p.FixupSectionSize, ""},
		{"Fixup Section Checksum", p.FixupSectionChecksum, ""},
		{"Loader Section Size", p.LoaderSectionSize, ""},
		{"Loader Section Checksum", p.LoaderSectionChecksum, ""},
		{"Object Table Offset", p.ObjectTableOffset, ""},
		{"Num Objects", p.NumObjects, ""},
		{"Object Page Table Offset", p.ObjectPageTableOffset, ""},
		{"Object Iter Page Table Offset", p.ObjectIterPageTableOffset, ""},
		{"Resource Table Offset", p.ResourceTableOffset, ""},
		{"Num Resource Table Entries", p.NumResourceTableEntries, ""},
		{"Resident Name Table Offset", p.ResidentNameTableOffset, ""},
		{"Entry Table Offset", p.EntryTableOffset, ""},
		{"Module Directives Offset", p.ModuleDirectivesOffset, ""},
		{"Num Module Directives", p.NumModuleDirectives, ""},
		{"Fixup Page Table Offset", p.FixupPageTableOffset, ""},
		{"Fixup Record Offset", p.FixupRecordOffset, ""},
		{"Import Module Table Offset", p.ImportModuleTableOffset, ""},
		{"Import Module Entry Count", p.ImportModuleEntryCount, ""},
		{"Import Proc Table Offset", p.ImportProcTableOffset, ""},
		{"Per Page Checksum Offset", p.PerPageChecksumOffset, ""},
		{"Data Pages Offset", p.DataPagesOffset, ""},
		{"Num Preload Pages", p.NumPreloadPages, ""},
		{"Non ResName Table Offset", p.NonResNameTableOffset, ""},
		{"Non ResName Table Length", p.NonResNameTableLength, ""},
		{"Non ResName Table Checksum", p.NonResNameTableChecksum, ""},
		{"Auto DS Object", p.AutoDSObject, ""},
		{"Debug Info Offset", p.DebugInfoOffset, ""},
		{"Debug Info Length", p.DebugInfoLength, ""},
		{"Num Instance Preload", p.NumInstancePreload, ""},
		{"Num Instance Demand", p.NumInstanceDemand, ""},
		{"Heap Size", p.HeapSize, ""},
	})
}

// DumpText writes the program, in text format, to the writer.
func (p *Program) DumpText(w *bufio.Writer, prefix string) {
	nprefix := prefix + indentLevel
	w.WriteString(prefix)
	w.WriteString("Header:\n")
	p.ProgramHeader.DumpText(w, nprefix)
	w.WriteByte('\n')
	for i, obj := range p.Objects {
		w.WriteString(prefix)
		w.WriteString("Object ")
		w.WriteString(strconv.Itoa(i + 1))
		w.WriteString(":\n")
		obj.DumpText(w, nprefix)
		w.WriteByte('\n')
	}
}
