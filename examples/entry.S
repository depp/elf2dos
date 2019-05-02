// Entry point for DOS programs.
.text
.globl _start
_start:
	// DPMI: ES initially points to the program segment prefix. If you want
	// the command-line arguments you probably want to keep this around.
	push	%ds
	pop	%es
	call	main
	movb	$0x4c, %ah
	int	$0x21
