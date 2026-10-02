# lab-go-overflow

Test bench for runtime/secret on windows/amd64 (golang/go#81796). The probes
are throwaway programs that settle factual claims. Nothing here goes into the
Go tree as is; contributions go through Gerrit.

| Probe | Question |
|---|---|
| `ctxprobe` | Does `GetThreadContext` (+ `CONTEXT_XSTATE`) return planted GP, XMM, YMM and, where present, AVX-512 state of a suspended thread? |
| `scanprobe` | Can a parent find a secret in a child by live scan and by full-memory minidump, with a live canary? |
| `excprobe` | Where does Windows write the register state of a faulting goroutine? |
| `gsignal` | Is `mp.gsignal` nil on Windows? Needs a Go source tree: `sh gsignal/run.sh <goroot>` |

| `excg0` | Does anything register-derived land on g0 during exception dispatch? `sh excg0/run.sh <goroot>` |
| `rsprobe` | Is a call-free spin inside `secret.Do` preemptible? Needs `GOEXPERIMENT=runtimesecret` |

`patches/` holds the work-in-progress runtime change that enables `secret.Do`
on windows/amd64.

CI (Actions, run manually; each job logs the CPU it landed on):

- `probes`: runs the standalone probes.
- `patched-tree`: builds Go at a pinned commit, with or without `patches/`,
  then runs `runtime/secret`, the probes and `go test -short runtime`.
