# Changelog

All notable changes to this project are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [1.0.0] - 2026-05-26

### Changed
- **Serial backend swapped from `github.com/tarm/serial` to
  `go.bug.st/serial` v1.7.0.** `tarm/serial` has been unmaintained since
  2018; `go.bug.st/serial` is its actively-maintained successor. Ports are
  now opened with `serial.Open` + an explicit `serial.Mode{BaudRate,
  DataBits: 8, Parity: NoParity, StopBits: OneStopBit}` (8N1), and the read
  timeout is applied via `SetReadTimeout` after open (the port is closed if
  that call fails). **The public API of both `l77` and `sp1` is unchanged.**
- **Minimum Go version is now 1.26** — the floor required by
  `go.bug.st/serial` v1.7.0 (its module declares `go 1.26.0`) and the
  transitive `golang.org/x/sys` v0.43.0. The v0.2.1 `x/sys` pin that kept
  the Go 1.18 build is therefore obsolete and has been removed. The CI
  go-version matrix is updated accordingly.

## [0.2.1] - 2026-05-24

### Fixed
- **Restored the Go 1.18 build.** Pinned the transitive dependency
  `golang.org/x/sys` to v0.30.0; a later release (v0.43.0) requires Go
  1.25 — it imports the standard-library `slices` package — which broke
  building on the module's declared Go 1.18 minimum. The termios syscalls
  used by `tarm/serial` are stable across these `x/sys` versions, so the
  pin is behaviour-preserving.

## [0.2.0] - 2026-05-24

Reliability, concurrency, and testability pass. Public API signatures
are unchanged; several new accessors and safer shutdown behaviour are
added.

### Fixed
- **No goroutine/fd leak on Close.** Serial ports are now opened with a
  `ReadTimeout`, so `Run` wakes periodically and actually returns when
  `Close` is called instead of blocking forever in `Read` on an idle
  device. (Both `l77` and `sp1`.)
- **sp1: no more "send on closed channel" panic.** The write queue now
  signals shutdown via a separate channel, so a `Write`/`Print*` racing
  `Close` is a safe no-op (returns the number of bytes accepted) instead
  of crashing the process. Buffered bytes still drain on close.
- **sp1: commit-gap is honoured after back-to-back CRs.** The drainer
  re-arms the post-CR idle gap on every CR, so two consecutive
  `Execute()` jobs no longer send the next job's first byte too early.
- **sp1: serial write errors are no longer silently swallowed** -- they
  are logged (and short writes reported) via the device log function.
- **l77: data races fixed.** The parsed result fields, the escrow mode,
  and the runtime-settable callbacks are now guarded by a mutex, so the
  `Run` goroutine and the caller no longer race. Read results from
  another goroutine via the new `Get*` accessors.
- **l77: bounded currency-code buffer.** A version frame whose `#`
  terminator is lost no longer grows the buffer without limit; the frame
  is dropped after a small cap.

### Added
- `l77` accessors: `GetModel`, `GetManufacturer`, `GetSerialNumber`,
  `GetVersion`, `GetChecksum`, `GetCurrencyCode`, `GetSensorStatus`
  (lock-protected; safe to call while `Run` is active).
- GitHub Actions CI (build + vet + `-race` tests over Go 1.18/1.21/stable
  and a gofmt check).
- Native fuzz target `l77.FuzzFeedByte` and many unit tests; statement
  coverage rose from ~52%/~24% to ~81%/~73% (l77/sp1). The serial port
  is now an `io.ReadWriteCloser` so tests can inject a fake.
- Runnable package examples for pkg.go.dev.

### Changed
- The `LICENSE` text is now the exact canonical MIT (the previous file
  had a reworded clause that pkg.go.dev's detector did not recognise, so
  documentation would not render).
- `GenerateQRCode` clamps oversized payloads and uses clearer
  length-byte variable names (wire bytes unchanged).

[0.2.0]: https://github.com/ultramcu/go-ict/releases/tag/v0.2.0
