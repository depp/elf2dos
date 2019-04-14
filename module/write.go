package module

import (
	"encoding/binary"
	"io"
)

const (
	pageBits = 12
	pageSize = 1 << pageBits
)

var zeropage [pageSize]byte

func pagecount(size uint32) uint32 {
	return (size + pageSize - 1) >> pageBits
}

// =================================================================================================

type objdata struct {
	object []byte
	page   []byte
}

func (d *objdata) write(obj *Object, fixup []uint32, first, count uint32) {
	var od [4 * 6]byte
	binary.LittleEndian.PutUint32(od[:], obj.Size)
	binary.LittleEndian.PutUint32(od[4:], obj.Addr)
	binary.LittleEndian.PutUint32(od[8:], uint32(obj.Flags))
	if len(fixup) != 0 {
		binary.LittleEndian.PutUint32(od[12:], uint32(len(d.page)/4)+1)
		binary.LittleEndian.PutUint32(od[16:], uint32(len(fixup)))
		for _, idx := range fixup {
			d.page = append(d.page, 0, byte(idx>>8), byte(idx&0xff), 0)
		}
	}
	d.object = append(d.object, od[:]...)
}

// =================================================================================================

func appendFixup(f Fixup, data []byte) []byte {
	var d [9]byte
	d[0] = byte(f.SrcType)
	var flags byte
	binary.LittleEndian.PutUint16(d[2:], uint16(f.Src))
	d[4] = byte(f.Target.Obj)
	n := 5
	if f.Target.Off > 0x7fff {
		flags |= 0x10
		binary.LittleEndian.PutUint32(d[n:], uint32(f.Target.Off))
		n += 4
	} else {
		binary.LittleEndian.PutUint16(d[n:], uint16(f.Target.Off))
		n += 2
	}
	d[1] = flags
	return append(data, d[:n]...)
}

type fixupdata struct {
	pages   []byte
	records []byte
}

// write writes out fixup records. Returns fixup record indexes for each page in
// the object, truncated to exclude trailing zeroes.
func (d *fixupdata) write(size uint32, fixups []Fixup) []uint32 {
	if size == 0 {
		return nil
	}
	npage := int32(pagecount(size))

	// Find the number of pages that include all fixups.
	var maxOff int32 = -1
	for _, f := range fixups {
		off := f.Src + 3
		if off > maxOff {
			maxOff = off
		}
	}
	if maxOff < 0 {
		return nil
	}
	if n := (maxOff >> pageBits) + 1; n > npage {
		npage = n
	}

	// Assign fixups to pages, bucket sort
	idxs := make([]uint32, npage)
	for _, f := range fixups {
		var last int32 = -1
		for off := int32(0); off < 3; off += 3 {
			pi := (f.Src + off) >> pageBits
			if pi > last && pi < npage {
				idxs[pi]++
			}
		}
	}
	idxs = idxs[:npage]
	var total uint32
	for i, n := range idxs {
		idxs[i] = total
		total += n
	}
	assigned := make([]Fixup, total)
	for _, f := range fixups {
		var last int32 = -1
		for off := int32(0); off < 4; off += 4 {
			pi := (f.Src + off) >> pageBits
			if pi > last && pi < npage {
				idx := idxs[pi]
				idxs[pi] = idx + 1
				assigned[idx] = f
			}
		}
	}

	// Write out fixup data
	pages := d.pages
	records := d.records
	if len(pages) == 0 {
		pages = make([]byte, 4)
	}
	var pos uint32
	for pi, idx := range idxs {
		if pos == idx {
			idxs[pi] = 0
		}
		idxs[pi] = uint32(len(pages) / 4)
		pfixups := assigned[pos:idx]
		pos = idx
		base := int32(pi << pageBits)
		for _, f := range pfixups {
			f.Src -= base
			records = appendFixup(f, records)
		}
		var roff [4]byte
		binary.LittleEndian.PutUint32(roff[:], uint32(len(records)))
		pages = append(pages, roff[:]...)
	}
	d.pages = pages
	d.records = records
	return idxs
}

// =================================================================================================

type pagedata struct {
	count  uint32
	offset uint32
	data   [][]byte
}

func (d *pagedata) write(data []byte) (first, count uint32) {
	count = pagecount(uint32(len(data)))
	if count != 0 {
		first = d.count + 1
		if d.offset != 0 {
			d.data = append(d.data, zeropage[d.offset:])
		}
		d.data = append(d.data, data)
		d.offset = uint32(len(data)) & (pageSize - 1)
		d.count += count
	}
	return
}

// =================================================================================================

type datawriter struct {
	pos  uint32
	data [][]byte
}

func (w *datawriter) write(d []byte) {
	w.pos += uint32(len(d))
	w.data = append(w.data, d)
}

// =================================================================================================

func (p *Program) dumpBlocks() [][]byte {
	var objdata objdata
	var fixupdata fixupdata
	var pagedata pagedata
	for _, obj := range p.Objects {
		first, count := pagedata.write(obj.Data)
		fixup := fixupdata.write(obj.Size, obj.Fixups)
		objdata.write(obj, fixup, first, count)
	}
	var h [0xac]byte
	le := binary.LittleEndian
	h[0] = 'L'
	h[1] = 'E'
	le.PutUint16(h[0x08:], 2)                      // 386 or higher
	le.PutUint32(h[0x14:], pagedata.count)         // number of pages
	le.PutUint32(h[0x18:], uint32(p.Entry.Obj))    // EIP object number
	le.PutUint32(h[0x1c:], uint32(p.Entry.Off))    // EIP offset
	le.PutUint32(h[0x20:], uint32(p.Stack.Obj))    // ESP object number
	le.PutUint32(h[0x24:], uint32(p.Stack.Off))    // ESP address
	le.PutUint32(h[0x28:], pageSize)               // Page size, 4 KiB
	le.PutUint32(h[0x2c:], pagedata.offset)        // Bytes on last page
	le.PutUint32(h[0x44:], uint32(len(p.Objects))) // Number of objects

	var d datawriter
	d.write(h[:])
	start := d.pos
	le.PutUint32(h[0x40:], d.pos) // Object table offset
	d.write(objdata.object)
	le.PutUint32(h[0x48:], d.pos) // Page table offset
	d.write(objdata.page)
	le.PutUint32(h[0x38:], d.pos-start) // Loader section size
	start = d.pos
	le.PutUint32(h[0x68:], d.pos) // Fixup page table offset
	d.write(fixupdata.pages)
	le.PutUint32(h[0x6c:], d.pos) // Fixup record table offset
	d.write(fixupdata.records)
	le.PutUint32(h[0x30:], d.pos-start) // Fixup section size
	le.PutUint32(h[0x80:], d.pos)       // Data page offset
	for _, it := range pagedata.data {
		d.write(it)
	}
	return d.data
}

// WriteTo writes the program to a writer.
func (p *Program) WriteTo(w io.Writer) (int64, error) {
	var amt int64
	for _, d := range p.dumpBlocks() {
		n, err := w.Write(d)
		amt += int64(n)
		if err != nil {
			return amt, err
		}
	}
	return amt, nil
}
