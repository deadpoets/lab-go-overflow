# lab-go-overflow

Private test bench for runtime/secret on windows/amd64. The probes are
throwaway programs that settle factual claims. Nothing here goes into the Go
tree as is; contributions go through Gerrit.

| Probe | Question |
|---|---|
| `ctxprobe` | Does `GetThreadContext` (+ `CONTEXT_XSTATE`) return planted GP, XMM, YMM and, where present, AVX-512 state of a suspended thread? |
| `scanprobe` | Can a parent find a secret in a child by live scan and by full-memory minidump, with a live canary? |
| `excprobe` | Where does Windows write the register state of a faulting goroutine? |
| `gsignal` | Is `mp.gsignal` nil on Windows? Needs a Go source tree: `sh gsignal/run.sh <goroot>` |

CI: Actions, "probes", run manually. Each job logs the CPU it landed on.
