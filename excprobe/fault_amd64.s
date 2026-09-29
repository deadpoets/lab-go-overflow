#include "textflag.h"

// func loadAndFault(p *[16]byte)
// Loads *p into X1..X4, zeroes *p so the pattern exists only in registers,
// then faults on a nil load.
TEXT ·loadAndFault(SB),NOSPLIT,$0-8
	MOVQ	p+0(FP), AX
	MOVOU	0(AX), X1
	MOVOU	0(AX), X2
	MOVOU	0(AX), X3
	MOVOU	0(AX), X4
	MOVQ	$0, 0(AX)
	MOVQ	$0, 8(AX)
	XORQ	AX, AX
	MOVQ	0(AX), AX
	RET
