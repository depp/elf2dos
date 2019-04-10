package main

// An objFlag is a set of flags for an object in an LE/LX executable.
type objFlag uint32

const (
	objR     objFlag = 0x0001 // readable
	objW     objFlag = 0x0002 // writable
	objX     objFlag = 0x0004 // executable
	obj32Bit objFlag = 0x2000 // whether segment is for 32-bit code
)

// A srcType is a fixup source type. These values match the LE/LX exe values.
type srcType uint32

const (
	srcOffset32   srcType = 0x07 // absolute 32-bit offset
	srcRelative32 srcType = 0x08 // self-relative 32-bit offset
)

// A fixup describes how a single reference in an object should be fixed after
// it is loaded into memory.
type fixup struct {
	srcType srcType // type of source reference to fix
	src     int32   // source offset within object
	target  ref     // target, where the relocation points to
	add     int32   // value to add to offset
}

// An object is a region of memory to be loaded when the program is run.
type object struct {
	flags  objFlag // object flags and permissions
	size   uint32  // size of the region, in memory
	addr   uint32  // base address where the data assumes the region is loaded
	data   []byte  // data, length may be smaller than region size
	fixups []fixup // list of fixups to apply to data after loading
}

// A ref is a reference to an address in the program.
type ref struct {
	obj int32 // 1-based index of object containing target
	off int32 // offset within target
}

// A program is an LE/LX format executable.
type program struct {
	entry   ref       // initial value of EIP
	stack   ref       // initial value of ESP
	objects []*object // objects to load
}
