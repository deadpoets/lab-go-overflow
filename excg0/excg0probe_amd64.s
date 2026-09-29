// Temporary probe; copied into go/src/runtime by run.sh and removed after.

#include "textflag.h"

// func probeLoadAndFault(p *[16]byte)
// Loads *p into X1..X15 with the low byte replaced by the register number,
// zeroes *p, then faults on a nil load. After this, each pattern copy exists
// only in one register, and its first byte says which one.
TEXT ·probeLoadAndFault(SB),NOSPLIT,$0-8
	MOVQ	p+0(FP), AX
	MOVOU	0(AX), X0
#define LOADREG(n, xr) \
	MOVB	$n, 0(AX) \
	MOVOU	0(AX), xr
	LOADREG(1, X1)
	LOADREG(2, X2)
	LOADREG(3, X3)
	LOADREG(4, X4)
	LOADREG(5, X5)
	LOADREG(6, X6)
	LOADREG(7, X7)
	LOADREG(8, X8)
	LOADREG(9, X9)
	LOADREG(10, X10)
	LOADREG(11, X11)
	LOADREG(12, X12)
	LOADREG(13, X13)
	LOADREG(14, X14)
	MOVQ	$0, 0(AX)
	MOVQ	$0, 8(AX)
	XORQ	AX, AX
	MOVQ	0(AX), AX
	RET
