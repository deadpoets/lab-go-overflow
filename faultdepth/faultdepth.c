// faultdepth: how far below the faulting SP the first user-mode exception
// handler finds itself, and how large the kernel's CONTEXT is, for a hardware
// fault and for RaiseException. Builds with MSVC on x64, x86 and ARM64.
#include <windows.h>
#include <stdio.h>
#include <stdlib.h>

static volatile int which;

static ULONG_PTR ctxsp(CONTEXT *c) {
#if defined(_M_ARM64)
	return c->Sp;
#elif defined(_M_X64)
	return c->Rsp;
#else
	return c->Esp;
#endif
}

static LONG CALLBACK handler(EXCEPTION_POINTERS *ep) {
	volatile char local;
	CONTEXT *c = ep->ContextRecord;
	ULONG_PTR sp = ctxsp(c);
	ULONG_PTR here = (ULONG_PTR)&local;
	DWORD all = 0, xs = 0;
	if (c->ContextFlags & CONTEXT_XSTATE) {
		CONTEXT_EX *ex = (CONTEXT_EX *)(c + 1);
		all = ex->All.Length;
		xs = ex->XState.Length;
	}
	printf("  %-16s code=%#lx ContextFlags=%#lx CONTEXT_EX.All.Length=%lu XState.Length=%lu\n",
	       which == 0 ? "hardware fault" : "RaiseException", ep->ExceptionRecord->ExceptionCode,
	       c->ContextFlags, all, xs);
	printf("  %-16s fault SP - handler local = %llu bytes; CONTEXT at SP-%llu, EXCEPTION_RECORD at SP-%llu\n",
	       "", (unsigned long long)(sp - here), (unsigned long long)(sp - (ULONG_PTR)c),
	       (unsigned long long)(sp - (ULONG_PTR)ep->ExceptionRecord));
	fflush(stdout);
	if (which == 0) {
		which = 1;
		RaiseException(0xE0000001, 0, 0, NULL);
	}
	ExitProcess(0);
	return EXCEPTION_CONTINUE_SEARCH;
}

int main(void) {
	DWORD n = 0;
#if defined(_M_ARM64)
	const char *arch = "arm64";
#elif defined(_M_X64)
	const char *arch = "amd64";
#else
	const char *arch = "386";
#endif
	printf("%s: sizeof(CONTEXT)=%zu sizeof(EXCEPTION_RECORD)=%zu CONTEXT_ALL=%#lx CONTEXT_XSTATE=%#lx\n",
	       arch, sizeof(CONTEXT), sizeof(EXCEPTION_RECORD), (unsigned long)CONTEXT_ALL, (unsigned long)CONTEXT_XSTATE);
	InitializeContext(NULL, CONTEXT_ALL | CONTEXT_XSTATE, NULL, &n);
	printf("  InitializeContext(ALL|XSTATE) errno=%lu n=%lu\n", GetLastError(), n);
	n = 0;
	InitializeContext(NULL, CONTEXT_ALL, NULL, &n);
	printf("  InitializeContext(ALL) errno=%lu n=%lu\n", GetLastError(), n);
	fflush(stdout);
	AddVectoredExceptionHandler(1, handler);
	which = 0;
	*(volatile int *)0x1000 = 1; // hardware fault
	return 1;
}
