package module_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"moria.us/elf2dos/module"
)

func TestProgramHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, new(module.ProgramHeader)); err != nil {
		t.Error("binary.Write:", err)
		return
	}
	const expectSize = 0xac
	size := buf.Len()
	if size != expectSize {
		t.Errorf("binary.Write: got %d, expected %d", size, expectSize)
	}
}
