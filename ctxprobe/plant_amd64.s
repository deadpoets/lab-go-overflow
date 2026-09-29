#include "textflag.h"

// func plantAndSpin(pat *[32]byte, ready, stop *uint32)
// Loads known values into BX, R12, R13, X1 and Y5, sets *ready, then spins
// with no calls until *stop is non-zero. Assembly is never an async
// preemption point, so the planted values stay put while suspended.
TEXT ·plantAndSpin(SB),NOSPLIT,$0-24
	MOVQ	pat+0(FP), AX
	MOVQ	ready+8(FP), CX
	MOVQ	stop+16(FP), DX
	MOVQ	0(AX), BX
	MOVQ	8(AX), R12
	MOVQ	16(AX), R13
	MOVOU	0(AX), X1
	VMOVDQU	0(AX), Y5
	MOVL	$1, (CX)
loop:
	PAUSE
	CMPL	(DX), $0
	JEQ	loop
	VZEROUPPER
	RET

// func plantAndSpin512(pat *[64]byte, ready, stop *uint32)
// Like plantAndSpin, but also loads the full 64 bytes into Z5 (upper half
// lands in ZMM_H) and Z16 (Hi16_ZMM), and 8 bytes into K2 (opmask).
// Only call when the CPU and OS support AVX-512.
TEXT ·plantAndSpin512(SB),NOSPLIT,$0-24
	MOVQ	pat+0(FP), AX
	MOVQ	ready+8(FP), CX
	MOVQ	stop+16(FP), DX
	VMOVDQU64	0(AX), Z5
	VMOVDQU64	0(AX), Z16
	KMOVQ	0(AX), K2
	MOVL	$1, (CX)
loop512:
	PAUSE
	CMPL	(DX), $0
	JEQ	loop512
	VZEROUPPER
	VPXORQ	Z16, Z16, Z16
	KXORQ	K2, K2, K2
	RET
