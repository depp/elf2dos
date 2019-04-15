package module

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

func readDataSection(fp *os.File, soffset, ssize uint32, doffset, dsize uint32) ([]byte, error) {
	if doffset < soffset || soffset+ssize-doffset < dsize {
		return nil, fmt.Errorf("range 0x%x:0x%x is outside section 0x%0x:0x%0x",
			doffset, uint64(doffset)+uint64(dsize), soffset, soffset+ssize)
	}
	data := make([]byte, dsize)
	if _, err := fp.ReadAt(data, int64(doffset)); err != nil {
		return nil, err
	}
	return data, nil
}

func deserialize(raw []byte, data interface{}) error {
	return binary.Read(bytes.NewReader(raw), binary.LittleEndian, data)
}

type section struct {
	name   string
	offset uint32
	size   uint32
}

type reader struct {
	fp     *os.File
	fsize  int64
	loader section
	fixup  section
}

func (r *reader) setSection(s *section, name string, offset, size uint32) error {
	if int64(offset) > r.fsize || int64(size) > r.fsize-int64(offset) {
		return fmt.Errorf("%s (offsets 0x%x:0x%x) extends beyond end of file (offset 0x%x)",
			offset, int64(offset)+int64(size), r.fsize)
	}
	*s = section{
		name:   name,
		offset: offset,
		size:   size,
	}
	return nil
}

func (r *reader) read(s *section, doffset, dsize uint32) ([]byte, error) {
	if int64(doffset) > r.fsize || int64(dsize) > r.fsize-int64(doffset) {
		return nil, fmt.Errorf("range 0x%x:0x%x is outside file 0x0:0x%0x",
			doffset, doffset+dsize, r.fsize)
	}
	data := make([]byte, dsize)
	if _, err := r.fp.ReadAt(data, int64(doffset)); err != nil {
		return nil, err
	}
	return data, nil
}

func (r *reader) readProgramHeader() (h ProgramHeader, err error) {
	// Read program header. (loader.asm:load_header)
	// DOS/32A assembly note:
	// _app_num_objects    = 0x44 NumObjects
	// _app_off_objects    = 0x40 ObjectTableOffset
	// _app_off_objpagetab = 0x48 ObjectPageTableOffset
	// _app_off_fixpagetab = 0x68 FixupPageTableOffset
	// _app_off_fixrectab  = 0x6c FixupRecordOffset
	// _app_off_datapages  = 0x80 DataPagesOffset
	// _app_siz_fixrecstab = 0x30 FixupSectionSize
	// _app_siz_lastpage   = 0x2c LastPageSize
	data := make([]byte, 0xac)
	if _, err := r.fp.ReadAt(data, 0); err != nil {
		if err == io.EOF {
			return h, io.ErrUnexpectedEOF
		}
		return h, err
	}
	if err := deserialize(data, &h); err != nil {
		return h, err
	}
	return h, nil
}

func (r *reader) readObjectTable(p *Program) error {
	// Read object table. (loader.asm:load_object)
	data, err := r.read(&r.loader, p.ObjectTableOffset, p.NumObjects*0x18)
	if err != nil {
		return err
	}
	ohdrs := make([]ObjectHeader, p.NumObjects)
	if err := deserialize(data, ohdrs); err != nil {
		return err
	}
	objs := make([]*Object, p.NumObjects)
	for i, h := range ohdrs {
		objs[i] = &Object{ObjectHeader: h}
	}
	p.Objects = objs
	return nil
}

func (r *reader) readObjectPageTable(p *Program) error {
	var count uint32
	for i, obj := range p.Objects {
		if obj.NumPageTableEntries != 0 && obj.PageTableIndex != 0 {
			ofirst := uint64(obj.PageTableIndex - 1)
			ocount := uint64(obj.NumPageTableEntries)
			oend := ofirst + ocount
			if oend*4 > uint64(^uint32(0)) {
				return fmt.Errorf("object %d has invalid page table range", i+1)
			}
			if uint32(oend) > count {
				count = uint32(oend)
			}
		}
	}
	data, err := r.read(&r.loader, p.ObjectPageTableOffset, count*4)
	if err != nil {
		return err
	}
	hdrs := make([]ObjectPageHeader, count)
	if err := binary.Read(bytes.NewReader(data), binary.BigEndian, hdrs); err != nil {
		return err
	}
	table := make([]*ObjectPage, count)
	for i, h := range hdrs {
		table[i] = &ObjectPage{ObjectPageHeader: h}
	}
	for _, obj := range p.Objects {
		if obj.NumPageTableEntries != 0 && obj.PageTableIndex != 0 {
			obj.Pages = table[obj.PageTableIndex-1 : obj.PageTableIndex-1+obj.NumPageTableEntries]
		}
	}
	return nil
}

func (r *reader) readFixupPageTable(p *Program) ([]uint32, error) {
	var maxIndex uint32
	for _, obj := range p.Objects {
		for _, p := range obj.Pages {
			if idx := uint32(p.FixupPageIndex); idx > maxIndex {
				maxIndex = idx
			}
		}
	}
	if maxIndex == 0 {
		return nil, nil
	}
	data, err := r.read(&r.fixup, p.FixupPageTableOffset, 4*(maxIndex+1))
	if err != nil {
		return nil, err
	}
	offsets := make([]uint32, maxIndex+1)
	if err := deserialize(data, offsets); err != nil {
		return nil, err
	}
	var last uint32
	for _, x := range offsets {
		if x < last {
			return nil, errors.New("fixup offset table jumps backwards")
		}
		last = x
	}
	return offsets, nil
}

var errShortFixup = errors.New("unexpected end of table")

func readFixup(data []byte) (n int, fix Fixup, err error) {
	if len(data) < 7 {
		return 0, Fixup{}, errShortFixup
	}
	src := data[0]
	flags := data[1]
	srcoff := int16(binary.LittleEndian.Uint16(data[2:]))
	if src&0x20 != 0 {
		// Also unimplemented by DOS/32A
		return 0, Fixup{}, fmt.Errorf("source list fixups unimplemented (srctype = 0x%02x)", src)
	}
	if flags&0x03 != 0 {
		return 0, Fixup{}, fmt.Errorf("imported fixups unimplemented (flags = 0x%02x)", flags)
	}
	if flags&0x04 != 0 {
		return 0, Fixup{}, fmt.Errorf("additive fixups unimplemented (flags = 0x%02x)", flags)
	}
	var objnum uint16
	if flags&0x40 != 0 {
		// 16-bit object number
		objnum = binary.LittleEndian.Uint16(data[4:])
		data = data[6:]
		n = 6
	} else {
		objnum = uint16(data[4])
		data = data[5:]
		n = 5
	}
	if t := src & 0x0f; t > 8 {
		return 0, Fixup{}, fmt.Errorf("unimplemented source type %d", t)
	}
	var target int32
	if flags&0x10 != 0 {
		if len(data) < 4 {
			return 0, Fixup{}, errShortFixup
		}
		target = int32(binary.LittleEndian.Uint32(data))
		data = data[4:]
		n += 4
	} else {
		if len(data) < 2 {
			return 0, Fixup{}, errShortFixup
		}
		target = int32(binary.LittleEndian.Uint16(data))
		data = data[2:]
		n += 2
	}
	fix = Fixup{
		SrcType: SrcType(src),
		Src:     int32(srcoff),
		Target: Ref{
			Obj: int32(objnum),
			Off: target,
		},
		Add: 0,
	}
	return n, fix, nil
}

func (r *reader) readFixupRecords(p *Program, pageTable []uint32) error {
	if len(pageTable) == 0 {
		return nil
	}
	data, err := r.read(&r.fixup, p.FixupRecordOffset, pageTable[len(pageTable)-1])
	if err != nil {
		return err
	}
	count := len(pageTable) - 1
	pageFixups := make([][]Fixup, count)
	for i := range pageFixups {
		off0 := pageTable[i]
		off1 := pageTable[i+1]
		if off0 == off1 {
			continue
		}
		var fixups []Fixup
		fdata := data[off0:off1]
		for len(fdata) != 0 {
			n, fix, err := readFixup(fdata)
			if err != nil {
				return fmt.Errorf("invalid fixup at file offset 0x%0x: %v",
					p.FixupRecordOffset+off1-uint32(len(fdata)), err)
			}
			fixups = append(fixups, fix)
			fdata = fdata[n:]
		}
		pageFixups[i] = fixups
	}
	for _, obj := range p.Objects {
		for _, p := range obj.Pages {
			if p.FixupPageIndex != 0 {
				p.Fixups = pageFixups[p.FixupPageIndex-1]
			}
		}
	}
	return nil
}

func (r *reader) readObjectData(obj *Object, offset, lastPageSize uint32) (uint32, error) {
	if obj.NumPageTableEntries == 0 {
		return 0, nil
	}
	dataSize := ((obj.NumPageTableEntries - 1) << PageBits) + lastPageSize
	if obj.VirtualSize < dataSize {
		dataSize = obj.VirtualSize
	}
	rem := r.fsize - int64(offset)
	if int64(dataSize) > rem {
		return 0, fmt.Errorf(
			"object data (offsets 0x%x:0x%x) extends past end of file (offset 0x%x)",
			offset, int64(offset)+int64(dataSize), r.fsize)
	}
	data := make([]byte, dataSize)
	if _, err := r.fp.ReadAt(data, int64(offset)); err != nil {
		return 0, err
	}
	obj.Data = data
	return dataSize, nil
}

func (r *reader) readProgram() (*Program, error) {
	h, err := r.readProgramHeader()
	if err != nil {
		return nil, fmt.Errorf("could not read program header: %v", err)
	}
	if !h.IsLE() {
		return nil, fmt.Errorf("unknown program signature %q (expected LE)", h.Signature[:])
	}
	if h.PageSize != PageSize {
		return nil, fmt.Errorf("unsupported page size: %d", h.PageSize)
	}
	if h.LastPageSize == 0 || h.LastPageSize > PageSize {
		return nil, fmt.Errorf("invalid last page size: %d", h.LastPageSize)
	}
	const maxObjects = 64
	if h.NumObjects > 64 {
		return nil, fmt.Errorf("too many objects: %d (maximum: %d)", h.NumObjects, maxObjects)
	}
	if err := r.setSection(&r.loader, "loader section",
		h.ObjectTableOffset, h.LoaderSectionSize); err != nil {
		return nil, err
	}
	if err := r.setSection(&r.fixup, "fixup section",
		h.FixupPageTableOffset, h.FixupSectionSize); err != nil {
		return nil, err
	}
	if int64(h.DataPagesOffset) > r.fsize {
		return nil, fmt.Errorf(
			"start of data pages (offset 0x%x) are past end of file (offset 0x%x)",
			h.DataPagesOffset, r.fsize)
	}
	p := Program{ProgramHeader: h}
	if err := r.readObjectTable(&p); err != nil {
		return nil, fmt.Errorf("could not read object table: %v", err)
	}
	if err := r.readObjectPageTable(&p); err != nil {
		return nil, fmt.Errorf("could not read object page table: %v", err)
	}
	fixupPageTable, err := r.readFixupPageTable(&p)
	if err != nil {
		return nil, fmt.Errorf("could not read fixup page table: %v", err)
	}
	if err := r.readFixupRecords(&p, fixupPageTable); err != nil {
		return nil, fmt.Errorf("could not read fixup records: %v", err)
	}
	var lastObject int
	for i, obj := range p.Objects {
		if obj.NumPageTableEntries != 0 {
			lastObject = i
		}
	}
	dataOffset := h.DataPagesOffset
	for i, obj := range p.Objects {
		var lastPageSize uint32 = PageSize
		if i == lastObject {
			lastPageSize = h.LastPageSize
		}
		n, err := r.readObjectData(obj, dataOffset, lastPageSize)
		if err != nil {
			return nil, fmt.Errorf("could not read object %d data: %v", i+1, err)
		}
		dataOffset += n
	}
	return &p, nil
}

// Open opens that named file with os.Open and reads the LE module structure.
func Open(name string) (*Program, error) {
	// We follow the same way that DOS/32A reads the executables, so we can be
	// as compatible as possible.
	fp, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer fp.Close()
	st, err := fp.Stat()
	if err != nil {
		return nil, err
	}
	r := reader{
		fp:    fp,
		fsize: st.Size(),
	}
	return r.readProgram()
}
