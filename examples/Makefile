all: hello.exe
clean:
	rm -f entry.o main.o hello.elf hello.le hello.exe
run: hello.exe
	dosbox -c 'MOUNT C: .' -c 'C:' -c 'HELLO.EXE'

.PHONY: all clean run

ASFLAGS := -m32
CFLAGS := -m32 -ffreestanding -march=i386 -fno-pic -fno-pie -fno-exceptions -fno-unwind-tables -fno-asynchronous-unwind-tables -O2 -Wall -Wextra
LDFLAGS := -nostdlib -emit-relocs

entry.o: entry.S
main.o: main.c
hello.elf: link.ld entry.o main.o
	ld $(LDFLAGS) -script=link.ld -o $@ entry.o main.o
hello.le: hello.elf elf2dos
	./elf2dos -output $@ $<
elf2dos: $(match ../*.go ../module/*.go ../elf/*.go)
	go build ..
hello.exe: hello.le sb.exe
	@rm -f TEMP.EXE
	@cp hello.le TEMP.LE
	SDL_VIDEODRIVER=dummy dosbox -c 'MOUNT C: .' -c 'C:' -c 'SB.EXE /B /BNTEMP.EXE TEMP.LE >OUT' -c EXIT
	@cat >&2 OUT
	@rm -f OUT
	@test -f TEMP.EXE || exit 1
	@mv -f TEMP.EXE hello.exe
