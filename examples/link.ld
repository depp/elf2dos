/*
Linker script for 32-bit dos programs. The programs must then be processed with
elf2dos.

Note that everything is aligned to 0x10 (paragraph) alignment. DOS/32A loads
with paragraph alignment by default. We can't be bothered to change this, so we
just live with it.
*/

OUTPUT_ARCH(i386)
OUTPUT_FORMAT("elf32-i386", "elf32-i386", "elf32-i386")

SECTIONS
{
  ENTRY(_start)

  . = 0x10000;
  .text : {
    *(.text .text.*)
    *(.rodata .rodata.*)
  }

  . += 0x2000;
  .data : ALIGN(0x10) {
    *(.data .data.*)
  }
  .bss : ALIGN(0x10) {
    *(.bss .bss.*)
  }
  .stack : ALIGN(0x10) {
    . += 0x8000;
    _stack_end = .;
  }

  /DISCARD/ : {
    *(.note .note.*)
    *(.comment .comment.*)
  }
}
