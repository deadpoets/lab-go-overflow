#include "textflag.h"

// func holdInRegister(p *[16]byte, ready *uint32, stop *uint32)
// Loads *p into X1, zeroes *p, sets *ready and spins until *stop != 0.
// After the zeroing, the pattern exists only in X1.
TEXT ·holdInRegister(SB),NOSPLIT,$0-24
	MOVQ	p+0(FP), AX
	MOVQ	ready+8(FP), CX
	MOVQ	stop+16(FP), DX
	MOVOU	0(AX), X1
	MOVQ	$0, 0(AX)
	MOVQ	$0, 8(AX)
	MOVL	$1, (CX)
loop:
	PAUSE
	CMPL	(DX), $0
	JEQ	loop
	PXOR	X1, X1
	RET
