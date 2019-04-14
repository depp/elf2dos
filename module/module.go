// Package module provides an interface to LE linear executable modules.
package module

// An ObjFlag is a set of flags for an object in an LE/LX executable.
type ObjFlag uint32

const (
	// ObjR indicates a readable object
	ObjR ObjFlag = 0x0001
	// ObjW indicates a writable object
	ObjW ObjFlag = 0x0002
	// ObjX indicates an executable object
	ObjX ObjFlag = 0x0004
	// Obj32Bit indicates the object is 32-bit
	Obj32Bit ObjFlag = 0x2000
)

// A SrcType is a fixup source type. These values match the LE/LX exe values.
type SrcType uint32

const (
	// SrcOffset32 indicates an absolute 32-bit offset.
	SrcOffset32 SrcType = 0x07
	// SrcRelative32 indicates a self-relative 32-bit offset.
	SrcRelative32 SrcType = 0x08
)

// A Fixup describes how a single reference in an object should be fixed after
// it is loaded into memory.
type Fixup struct {
	SrcType SrcType // type of source reference to fix
	Src     int32   // source offset within object
	Target  Ref     // target, where the relocation points to
	Add     int32   // value to add to offset
}

// An Object is a region of memory to be loaded when the program is run.
type Object struct {
	Flags  ObjFlag // object flags and permissions
	Size   uint32  // size of the region, in memory
	Addr   uint32  // base address where the data assumes the region is loaded
	Data   []byte  // data, length may be smaller than region size
	Fixups []Fixup // list of fixups to apply to data after loading
}

// A Ref is a reference to an address in the program.
type Ref struct {
	Obj int32 // 1-based index of object containing target
	Off int32 // offset within target
}

// A Program is an LE/LX format executable.
type Program struct {
	Entry   Ref       // initial value of EIP
	Stack   Ref       // initial value of ESP
	Objects []*Object // objects to load
}
