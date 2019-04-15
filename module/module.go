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

// An ObjectHeader is the header for a loadable object in an LE/LX format
// executable.
type ObjectHeader struct {
	VirtualSize      uint32 // Size, in memory, in bytes
	BaseAddress      uint32 // Base address where the data assumes the region is loaded
	Flags            ObjFlag
	PageTableIndex   uint32 // 1-based offset into object page table
	PageTableEntries uint32 // Number of page table entries
	Reserved         uint32
}

// An Object is a region of memory to be loaded when the program is run.
type Object struct {
	ObjectHeader
	Data   []byte  // data, length may be smaller than region size
	Fixups []Fixup // list of fixups to apply to data after loading
}

// A Ref is a reference to an address in the program.
type Ref struct {
	Obj int32 // 1-based index of object containing target
	Off int32 // offset within target
}

// A ProgramHeader is the header for an LE/LX format executable.
type ProgramHeader struct {
	Signature                 [2]byte // "LE" or "LX"
	ByteOrder                 uint8
	WordOrder                 uint8
	FormatLevel               uint32
	CPUType                   uint16 // Minimum CPU type supported
	OSType                    uint16 // OS type supported
	ModuleVersion             uint32 // Version of this module
	ModuleFlags               uint32
	ModuleNumPages            uint32
	EIP                       Ref    // Initial value of EIP
	ESP                       Ref    // Initial value of ESP
	PageSize                  uint32 // Size of data pages
	LastPageSize              uint32 // Size of last page
	FixupSectionSize          uint32 // Size of fixup section
	FixupSectionChecksum      uint32 // Checksum of fixup section, or 0
	LoaderSectionSize         uint32 // Size of loader section
	LoaderSectionChecksum     uint32 // Checksum of loader section, or 0
	ObjectTableOffset         uint32 // Offset of object table, from header
	NumObjects                uint32 // Number of objects in the module
	ObjectPageTableOffset     uint32 // Object page table offset, from header
	ObjectIterPageTableOffset uint32 // Object iterated page table offset, from header
	ResourceTableOffset       uint32
	NumResourceTableEntries   uint32
	ResidentNameTableOffset   uint32
	EntryTableOffset          uint32
	ModuleDirectivesOffset    uint32
	NumModuleDirectives       uint32
	FixupPageTableOffset      uint32 // Fixup page table offset, from header
	FixupRecordOffset         uint32 // Fixup record offset, from header
	ImportModuleTableOffset   uint32 // Import module table offset, from header
	ImportModuleEntryCount    uint32 // Number of import module entries
	ImportProcTableOffset     uint32
	PerPageChecksumOffset     uint32
	DataPagesOffset           uint32 // Data pages offset, from header
	NumPreloadPages           uint32
	NonResNameTableOffset     uint32
	NonResNameTableLength     uint32
	NonResNameTableChecksum   uint32
	AutoDSObject              uint32
	DebugInfoOffset           uint32
	DebugInfoLength           uint32
	NumInstancePreload        uint32
	NumInstanceDemand         uint32
	HeapSize                  uint32
}

// IsLE returns true if the program header is for an LE executable.
func (p *ProgramHeader) IsLE() bool {
	return p.Signature[0] == 'L' && p.Signature[1] == 'E'
}

// IsLE returns true if the program header is for an LX executable.
func (p *ProgramHeader) IsLX() bool {
	return p.Signature[0] == 'L' && p.Signature[1] == 'X'
}

// A Program is an LE/LX format executable.
type Program struct {
	ProgramHeader
	Objects []*Object // objects to load
}
