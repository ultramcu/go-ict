# Changelog

All notable changes to this project are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

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
