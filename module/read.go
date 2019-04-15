package module

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// Open opens that named file with os.Open and reads the LE module structure.
func Open(name string) (*Program, error) {
	fp, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	st, err := fp.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()

	r := bufio.NewReader(fp)
	p := new(Program)
	if err := binary.Read(r, binary.LittleEndian, &p.ProgramHeader); err != nil {
		return nil, err
	}
	if !p.IsLE() && !p.IsLX() {
		return nil, fmt.Errorf("unknown program signature %q (expected LE or LX)", p.Signature[:])
	}

	// Read object table
	if p.NumObjects > 64 {
		return nil, fmt.Errorf("too many objects: %d", p.NumObjects)
	}
	if int64(p.ObjectTableOffset) > size ||
		int64(p.ObjectTableOffset)+int64(p.NumObjects)*0x18 > size {
		return nil, errors.New("object table is out of bounds")
	}
	if _, err := fp.Seek(int64(p.ObjectTableOffset), io.SeekStart); err != nil {
		return nil, err
	}
	ohdrs := make([]ObjectHeader, p.NumObjects)
	r.Reset(fp)
	if err := binary.Read(r, binary.LittleEndian, ohdrs); err != nil {
		return nil, err
	}
	objs := make([]*Object, p.NumObjects)
	for i, h := range ohdrs {
		objs[i] = &Object{ObjectHeader: h}
	}
	p.Objects = objs

	return p, nil
}
