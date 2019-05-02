# Elf2Dos

Cross-compile a 32-bit DOS program with GCC!

- Use any compiler that targets x86 ELF. This means you can use the version of GCC or Clang already installed on your x86 Linux system.

- Write 32-bit protected mode code. You do not have to worry about near or far pointers, and you are not limited to 64K.

This is a horribly ill-advised way to write code and it will surely erode your sanity. You just have to write a DOS program without using anything in the standard library, and this program will convert it into a 32-bit LE “Linear Executable” which can be loaded with [DOS/32 Advanced][dos32a]. The input must be a 32-bit ELF executable with the relocations preserved. The GNU linker will do this with the `--emit-relocs` flag. Only `R_386_32` and `R_386_PC32` relocations are supported.

[dos32a]: http://dos32a.narechk.net/index_en.html

An example “Hello, World!” program is available in the [examples](examples) directory.

A blog post is forthcoming.

## Getting This to Work

- You should probably be using DJGPP instead of Elf2Dos.

- For obvious reasons you cannot use the standard library. Link with `-nostdlib` and compile with `-ffreestanding`.

- GCC will still emit calls to `memcpy`, `memmove`, `memset`, and `memcmp`, so you may need to define these.

- A linker script is a good idea. An example is available in [examples/link.ld](examples/link.ld).

- The `_stack_end` symbol defines the initial value of the stack pointer `esp`. This must be defined in the input. A natural way to do this is with a linker script,

  ```
  .stack : ALIGN(0x10) {
      _stack_start = .;
      . += 0x8000;
      _stack_end = .;
  }
  ```

- The `es` segment will refer to the PSP at program start. Copy `ds` to `es` at some point or your string instructions won’t work.

- DOS/32 Advanced by default uses 16-byte alignment. Don’t bother aligning anything to pages unless you change that.

## Future Work

- Combine executable with stub without having to run DOSBox.

## License

Elf2Dos is licensed under the terms of the MIT license. See [LICENSE.txt](LICENSE.txt) for details.
