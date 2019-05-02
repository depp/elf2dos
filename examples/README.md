# Example Programs

This demo shows how to cross-compile a 32-bit DOS program.

## Requirements

- C build tools for ELF binaries, including a C compiler, assembler, linker, and Make. Tested with GNU ld and GCC 8.3. Clang _should_ work, but the build scripts may require some modification.

- [Go 1.11][go] or higher to build Elf2Dos (modules support required).

- [DOSBox][dosbox].

- [DOS/32 Advanced][dos32a]: Copy `sb.exe` and `dos32a.exe` to this directory.

[go]: https://golang.org/
[dosbox]: https://www.dosbox.com/
[dos32a]: http://dos32a.narechk.net/index_en.html

## Building

To build `hello.exe`,

    make

To run `hello.exe` inside DOSBox,

    make run
