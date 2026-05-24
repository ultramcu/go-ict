# go-ict

[![CI](https://github.com/ultramcu/go-ict/actions/workflows/ci.yml/badge.svg)](https://github.com/ultramcu/go-ict/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ultramcu/go-ict.svg)](https://pkg.go.dev/github.com/ultramcu/go-ict)
[![Go Report Card](https://goreportcard.com/badge/github.com/ultramcu/go-ict)](https://goreportcard.com/report/github.com/ultramcu/go-ict)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Go drivers for [ICT](https://www.ictgroup.com.tw) kiosk peripherals
talking over an RS-232 serial port.

| Sub-package | Device | Vendor page |
| --- | --- | --- |
| [`l77`](./l77) | ICT **L77** bill acceptor | <https://www.ictgroup.com.tw/pro_cen.php?prod_id=16> |
| [`sp1`](./sp1) | ICT **SP1** thermal receipt printer | <https://www.ictgroup.com.tw/pro_cen.php?prod_id=124> |

Both devices are common in self-service kiosks, ticket vending
machines, and bill-payment boxes. The drivers are pure Go on top of
[`github.com/tarm/serial`](https://github.com/tarm/serial), with no
other third-party dependencies.

## Install

```sh
go get github.com/ultramcu/go-ict
```

Requires Go 1.18 or newer.

## L77 — bill acceptor

```go
package main

import (
    "log"
    "time"

    "github.com/ultramcu/go-ict/l77"
)

func main() {
    dev, err := l77.New(
        "/dev/ttyUSB0",
        onEscrow,        // bill arrived in the escrow path
        onStack,         // bill stacked
        onReject,        // bill rejected
        onStatus,        // status / error frame
        l77.AutoHold,    // hold each bill until ReqBillAccept/Reject
        nil,             // optional log function (e.g. log.Printf)
    )
    if err != nil {
        log.Fatal(err)
    }
    defer dev.Close()

    go dev.Run()

    // Some devices need a moment after open before they'll accept
    // commands; the L77 reference app waits one second.
    time.Sleep(time.Second)
    dev.ReqReset()
    time.Sleep(time.Second)
    dev.ReqEnable()

    select {} // run forever
}

func onEscrow(d *l77.Device, b l77.BillType) {
    log.Printf("bill in escrow: %d THB", b.Value())
    if b.Value() >= 100 {
        d.ReqBillAccept()
    } else {
        d.ReqBillReject()
    }
}

func onStack(_ *l77.Device, b l77.BillType)  { log.Printf("stacked: %d", b.Value()) }
func onReject(_ *l77.Device, b l77.BillType) { log.Printf("rejected: %d", b.Value()) }
func onStatus(_ *l77.Device, s l77.Status)   { log.Printf("status: %s", s) }
```

The four escrow modes are:

| Mode | Behaviour |
| --- | --- |
| `l77.DoNothing` | The escrow callback decides; you must call `ReqBillAccept`, `ReqBillReject` or `ReqBillHold` yourself. |
| `l77.AutoAccept` | Accept every bill that gets through the device's own validation. |
| `l77.AutoReject` | Reject every bill (useful for "out of service" mode). |
| `l77.AutoHold` | Hold the bill in escrow as soon as it arrives, then run the escrow callback so you can decide whether to accept or reject. |

## SP1 — thermal printer

```go
package main

import (
    "log"

    "github.com/ultramcu/go-ict/sp1"
)

func main() {
    dev, err := sp1.New("/dev/ttyUSB1", 9600, nil)
    if err != nil {
        log.Fatal(err)
    }
    defer dev.Close()

    go dev.Run()
    dev.Init()

    dev.PrintfFormat(sp1.Large, sp1.Center, "Receipt #1234\n")
    dev.SetFontSize(sp1.Normal)
    dev.Printf("Item ............... 99.00 THB\n")
    dev.Printf("Tax ................. 7.00 THB\n")
    dev.PrintfFormat(sp1.Large, sp1.Right, "106.00\n")
    dev.LineFeed()
    dev.LineFeed()
    dev.SetQRCodeSize(5)
    dev.GenerateQRCode("https://example.com/receipt/1234")
    dev.LineFeed()
    dev.LineFeed()
    dev.LineFeed()
    dev.Execute() // CR -- commits the print job
}
```

Notes specific to the SP1's protocol:

- Print jobs are committed only when a `CR` (`0x0D`) byte is sent —
  call `dev.Execute()` at the end of every receipt.
- The printer needs a small idle period after a commit before it
  will accept the next batch reliably. The driver enforces this gap
  internally via a single drainer goroutine, so caller code can issue
  back-to-back `Printf` calls without sleeps.
- Font sizes are `Small` (~48 chars / line), `Normal` (~24), `Large`
  (~16). `PrintfFormat` software-aligns text inside the per-font
  budget so centred and right-aligned lines work at any size.

## Serial port permissions

On Linux the user running the program needs read/write access to the
serial device. Either add the user to the `dialout` group:

```sh
sudo usermod -aG dialout $USER
```

…then log out and back in, or `chmod 666` the device for testing.

On macOS USB-serial adapters appear as `/dev/cu.usbserial-…`; no
special permissions are usually needed.

## License

[MIT](LICENSE) — `SPDX-License-Identifier: MIT`.
