# Elf2Dos

This is a horribly inadvised program which converts ELF objects to DOS programs.

The input must be a 32-bit ELF executable with the relocations preserved. It must not contain any unsupported relocations; only `R_386_32` and `R_386_PC32` relacations are supported.

The output will be a 32-bit LE “Linear Executable” which can be loaded with DOS/32 Advanced.
