# gopushpixels

**gopushpixels** is an MIT-licensed high-performance Go support library for
[PixelPusher](http://www.heroicrobotics.com/products/pixelpusher) hardware.

[![GoDoc](https://godoc.org/github.com/danjacques/gopushpixels?status.svg)](http://godoc.org/github.com/danjacques/gopushpixels)
[![Build Status](https://travis-ci.org/danjacques/gopushpixels.svg?branch=master)](https://travis-ci.org/danjacques/gopushpixels)
[![Coverage Status](https://coveralls.io/repos/github/danjacques/gopushpixels/badge.svg?branch=master)](https://coveralls.io/github/danjacques/gopushpixels?branch=master)

**gopushpixels** is not an official package. This package was initially built
in order to help a friend with their light show display, and refined and made
public to offer Go support for this awesome device.

This package is conscious of memory and CPU usage, and is designed to run on a
Raspberry Pi. Many packet parsing options are zero-copy and/or support buffer
reuse.

This powers [PixelProxy](https://github.com/danjacques/pixelproxy/), software
which has been observed driving 15,000+ pixels at 40 FPS on a single Raspberry
Pi without coming close to hitting any hardware constraints.

## Capabilities

**gopushpixels** is a set of Go packages offering a fully-featured PixelPusher
interface, which can:

*   Passively discover devices and maintain a registry of active devices.
*   Automatically generate stubs to interact with discovered devices.
*   Generate, manipulate, and capture pixel buffers.
*   Efficiently route pixel data to devices by group/controller or ID.
*   Offers a man-in-the-middle proxy capability, which can:
    *   Intercept, inspect, record, and modify PixelPusher data.
    *   Advertise as fake PixelPusher devices, to interface with generation
        software.
*   Collect operational metrics using [Prometheus](https://prometheus.io/)
    client integration.
*   Support for several hardware devices, configurations, layouts, and software
    versions.
*   Facilities to record and replay pixel data.
*   A low-overhead file format to store pixel data:
    *   Data can be associated with local or physical devices.
    *   Smaller files can be merged together.
    *   Optional compression support.
*   Perform both generation and parsing of PixelPusher's protocol, allowing
    simulation of PixelPusher devices.

## Packages

**gopushpixels** includes a set of core packages, which expose basic device
interoperability and functionality. These packages attempt to minimize external
dependencies:

*   [device](./device), an abstraction of a PixelPusher device and
    constructs to manage, track, and interact with them.
*   [discovery](./discovery), utility classes to interact with the device's
    discovery announcements.
*   [protocol](./protocol), an expression of the PixelPusher's discovery,
    command, and data network protocols, and utilities to read and write to
    them.
*   [support](./support), auxiliary capabilities used by the other packages.

### Features

**gopushpixels** also includes some feature packages. These provide non-core
functionality for devices that may be useful.

*   [proxy](./proxy), a system to enable man-in-the-moddle operations on
    devices, creating local devices for each remote device which capture
    received data before forwarding it to the remote device.
*   [replay](./replay), which exposes the ability to record and replay packet
    streams, as well as a `streamfile`, a versatile packet file format designed
    to accommodate large amounts of pixel data efficiently.

Some higher-level libraries are instrumented with
[Prometheus](https://prometheus.io/) metrics. This is a low-overhead
instrumentation, and using Prometheus is entirely optional.

## License

**gopushpixels** is licensed under an MIT license. For more information, see
[LICENSE](./LICENSE).

## History

**gopushpixels** is an open-source version of a rapidly-developed
application called PixelProxy. PixelProxy is a full stack capable of:

*   Masquerading proxy devices for observed physical PixelPusher devices.
*   Recording observed pixels in a Google
    [protobuf](https://github.com/golang/protobuf/)-based file format capable
    of compression and composing files by merging file fragments.
*   HTTP-enabled control interface to manage files, record, playback, view
    latest-state snapshot visualizations, view device state, and view logs.
*   HTML/JS/CSS embedding for single-binary deployment.
*   Prometheus-instrumented `struct`s and
    [http/pprof](https://golang.org/pkg/net/http/pprof/) helpers for effective
    introspection and profiling.

PixelProxy was used to enable LED replay by the team that built the
[Baltimore Light City Octopus sculpture](
http://baltimore.cbslocal.com/2018/04/12/light-city-charlie-peacock-octopus/).

## Contributing

If you'd like to contribute a patch, please open an issue or submit a pull
request.

Submitted code must pass
[pre-commit-go](https://github.com/maruel/pre-commit-go) checks. These impose
basic formatting and correctness checks. Prior to developing code, please
install the `pcg` tool and Git hook:

```sh
go get -u github.com/maruel/pre-commit-go/cmd/... && pcg
```

Prior to submitting a pull request, please validate that it passes by executing
the Git hook and/or explicitly running `pcg` in the root of the repository:

```sh
pcg
```

## Resources

Most of this library was obtained by examining PixelPusher wire protocols and
examining code from the following libraries:

*   Java (canonical): https://github.com/robot-head/PixelPusher-java
*   Python: https://github.com/cagerton/pixelpie
*   Node: https://github.com/TheThingSystem/node-pixelpusher
*   C++: https://github.com/q-depot/Cinder-PixelPusher
*   PixelPusher Server: https://github.com/hzeller/pixelpusher-server

## TODO

*   It is a background task of mine to implement unit tests for important
    features in this package suite. **gopushpixels** uses the
    [Ginkgo](https://onsi.github.io/ginkgo) BDD-style testing framework, enabled
    by [Gomega](https://onsi.github.io/gomega) matchers.
