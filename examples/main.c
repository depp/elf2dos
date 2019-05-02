#include <stdbool.h>

enum {
    STDIN_FILENO = 0,
    STDOUT_FILENO = 1,
    STDERR_FILENO = 2,
};

static void print(const char *str, unsigned count) {
    bool success;
    __asm__ volatile(
        "movb $0x40,%%ah\n\t"
        "int $0x21"
        : "=@ccnc"(success)
        : "b"(STDOUT_FILENO), "c"(count), "d"(str)
        : "eax", "memory");
}

static const char kHello[] = "Hello, from Elf2Dos!";

int main(void) {
    print(kHello, sizeof(kHello) - 1);
    return 0;
}
