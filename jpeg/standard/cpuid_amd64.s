//go:build amd64 && !purego

#include "textflag.h"

// func cpuid(eax, ecx uint32) (a, b, c, d uint32)
TEXT ·cpuid(SB), NOSPLIT, $0-24
	MOVL eax+0(FP), AX
	MOVL ecx+4(FP), CX
	MOVQ BX, R11
	CPUID
	MOVL AX, a+8(FP)
	MOVL BX, b+12(FP)
	MOVL CX, c+16(FP)
	MOVL DX, d+20(FP)
	MOVQ R11, BX
	RET

// func xgetbv(ecx uint32) uint64
TEXT ·xgetbv(SB), NOSPLIT, $0-16
	MOVL ecx+0(FP), CX
	XGETBV
	SHLQ $32, DX
	ORQ DX, AX
	MOVQ AX, ret+8(FP)
	RET
