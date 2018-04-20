// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package device

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/support/logging"
	"github.com/danjacques/gopushpixels/support/network"

	"github.com/pkg/errors"
)

// remoteDeviceState is the state held by a remoteDevice. This state
// will be updated atomically within the device.
type remoteDeviceState struct {
	headers  *protocol.DiscoveryHeaders
	addr     net.Addr
	observed time.Time
}

// Remote is a device implementation for a remote PixelPusher device.
//
// Remote is typically backed by a discovered device's headers. In this case,
// Remote is a fully-functional local stub for that device and its most recent
// state.
//
// Remote may also be constructed directly as a stub with MakeRemoteStub. In
// this case, it will not have access to any headers, and is used solely as a
// local stub to the remote device.
type Remote struct {
	// Logger, if not nil, is the logger that this device and its supporting
	// constructs will use.
	Logger logging.L

	// We lock around these headers. They can be updated any time by a call
	// to "observe".
	state atomic.Value

	// closed when this device's discovery expires.
	doneC    chan struct{}
	doneOnce sync.Once

	// id is the device's self-reported ID, derived from its hardware address.
	id string

	// monitoring is the device's monitoring state.
	monitoring Monitoring

	// dispatcherMu controls the creation of our dispatcher singleton.
	dispatcherMu sync.Mutex
	// dispatcher is a singleton connection owned by this device. A dispatcher is
	// created when the first Sender is created and destroyed when the device is
	// marked Done.
	//
	// The Sender returned by the Remote's Sender method will use dispatcher to
	// perform its higher-level packet writing functionality. The dispatcher will
	// own its own connection to the device, which will be shut down when the
	// dispatcher is destroyed.
	//
	// It is responsible for interfacing between users (who send packets) and the
	// remote system, potentially shaping, optimizing, or throttling packets as
	// appropriate.
	//
	// A dispatcher fills the higher-level packet aspect of the Sender interface.
	//
	// dispatcher must be safe for concurrent use.
	dispatcher *packetDispatcher

	infoMu sync.Mutex
	// info is the latest device information.
	info Info
	// createTime is the time when this device was created.
	createdTime time.Time
}

var remoteDeviceType = &Remote{}

var _ D = (*Remote)(nil)

// MakeRemote initializes a Remote device instance.
//
// The device must not be used until it has been observed via Observe(), at
// which point it will become fully active and valid.
func MakeRemote(id string, dh *protocol.DiscoveryHeaders) *Remote {
	d := makeRemoteDevice(id)
	d.UpdateHeaders(d.createdTime, dh)
	return d
}

// MakeRemoteStub constructs a new Remote device without requiring a full
// set of headers.
//
// MakeRemoteStub can be used to communicate with devices at known addresses.
func MakeRemoteStub(id string, addr *net.UDPAddr) *Remote {
	d := makeRemoteDevice(id)
	d.setState(&remoteDeviceState{
		headers:  nil,
		addr:     addr,
		observed: time.Now(),
	})
	return d
}

func makeRemoteDevice(id string) *Remote {
	return &Remote{
		doneC:       make(chan struct{}),
		id:          id,
		createdTime: time.Now(),
	}
}

// UpdateHeaders live-updates this device's headers.
//
// This can be used to update an instance of the device that has been observed
// with a new set of headers (e.g., via discovery).
func (d *Remote) UpdateHeaders(now time.Time, dh *protocol.DiscoveryHeaders) {
	d.setState(&remoteDeviceState{
		headers:  dh,
		addr:     dh.Addr(),
		observed: now,
	})
	d.monitoring.Update(d)
}

func (d *Remote) setState(rds *remoteDeviceState) { d.state.Store(rds) }

func (d *Remote) getState() *remoteDeviceState {
	return d.state.Load().(*remoteDeviceState)
}

func (d *Remote) String() string {
	st := d.getState()
	if st.headers == nil {
		return fmt.Sprintf("%q @%v", d.id, st.addr)
	}
	return fmt.Sprintf("%q @%v (%v)", d.id, st.addr, st.headers.DeviceType)
}

// ID implements D.
func (d *Remote) ID() string { return d.id }

// Ordinal implements D.
func (d *Remote) Ordinal() Ordinal {
	st := d.getState()

	var ord Ordinal
	if st.headers != nil {
		if pp := st.headers.PixelPusher; pp != nil {
			ord.Group = int(pp.GroupOrdinal)
			ord.Controller = int(pp.ControllerOrdinal)
		}
	}
	return ord
}

// Sender implements D.
func (d *Remote) Sender() (Sender, error) {
	// Make sure that we have an active Dispatcher, and retain it.
	disp, err := d.ensureAndRetainDispatcher()
	if err != nil {
		return nil, err
	}

	// Create a base (raw) connection to our underlying device.
	//
	// The dispatcher's reference will be Released when the Sender is Closed.
	var s Sender
	s = &remoteSender{
		packetDispatcher: disp,
		d:                d,
	}
	s = MonitorSender(d, s)

	return s, nil
}

func (d *Remote) ensureAndRetainDispatcher() (*packetDispatcher, error) {
	d.dispatcherMu.Lock()
	defer d.dispatcherMu.Unlock()

	// Check if someone else instantiated the singleton in between our lock
	// acquisitions.
	if d.dispatcher != nil {
		d.dispatcher.Retain()
		return d.dispatcher, nil
	}

	// Create a new datagram sender for this device.
	rds := remoteDynamicDatagramSender{d: d}
	if err := rds.ensureSenderConnected(); err != nil {
		return nil, err
	}

	// Create a new dispatcher.
	d.dispatcher = &packetDispatcher{
		d:          d,
		logger:     logging.Must(d.Logger),
		onShutdown: d.clearDispatcher,
		sender:     &rds,
	}
	if err := d.dispatcher.RetainAndStart(); err != nil {
		return nil, err
	}
	return d.dispatcher, nil
}

// clearDispatcher clears the Remote's dispatcher. It is called by the
// packetDispatcher's onShutdown callback.
//
// A new dispatcher will be created when the next Sender is instantiated.
//
// As added protection, we first ensure that the dispatcher that we are clearing
// matches the current dispatcher. This could be false if users are sloppy
// with their Closes.
//
// Either way, the dispatcher is responsible for shutting itself down.
func (d *Remote) clearDispatcher(disp *packetDispatcher) {
	d.dispatcherMu.Lock()
	defer d.dispatcherMu.Unlock()

	if d.dispatcher == disp {
		// The dispatchers match, so clear the current reference.
		d.dispatcher = nil
	}
}

// DiscoveryHeaders implements D.
func (d *Remote) DiscoveryHeaders() *protocol.DiscoveryHeaders {
	if dh := d.getState().headers; dh != nil {
		return dh
	}
	return &protocol.DiscoveryHeaders{}
}

// DoneC implements D.
func (d *Remote) DoneC() <-chan struct{} { return d.doneC }

// Addr implements D.
func (d *Remote) Addr() net.Addr {
	return d.getState().addr
}

// Info implements D.
func (d *Remote) Info() (i Info) {
	state := d.getState()

	d.modInfo(func(di *Info) {
		i = Info{
			PacketsReceived: di.PacketsReceived,
			BytesReceived:   di.BytesReceived,

			PacketsSent: di.PacketsSent,
			BytesSent:   di.BytesSent,

			Created:  d.createdTime,
			Observed: state.observed,
		}
	})
	return
}

func (d *Remote) modInfo(fn func(*Info)) {
	d.infoMu.Lock()
	defer d.infoMu.Unlock()
	fn(&d.info)
}

// MarkDone closes this device's done channel, shutting down any observation
// and marking this device "done" to external users.
//
// MarkDone is safe for concurrent use, and may be called multiple times;
// however, calls past the first time will do nothing.
func (d *Remote) MarkDone() {
	d.doneOnce.Do(func() { close(d.doneC) })
	d.monitoring.Update(d)
}

// remoteDynamicDatagramSender is a network.DatagramSender that sends datagrams
// to a Remote device.
//
// Because a Remote device can receive header updates, it's possible for its
// address and port to change dynamically. remoteDynamicDatagramSender
// accommodates this by transparently opening a new connection if such a change
// is observed.
//
// remoteDynamicDatagramSender is not safe for concurrent use.
type remoteDynamicDatagramSender struct {
	d *Remote

	base     network.DatagramSender
	baseAddr *net.UDPAddr

	// When we create a new base, we record its datagram size and report it
	// here. This prevents us from needing to potentially create a new connection
	// when users call the otherwise-lightweight MaxDatagramSize.
	lastDatagramSize int
}

func (rds *remoteDynamicDatagramSender) Close() error {
	if rds.base == nil {
		return nil
	}

	ds := rds.base
	rds.base, rds.baseAddr = nil, nil
	return ds.Close()
}

func (rds *remoteDynamicDatagramSender) SendDatagram(d []byte) error {
	if err := rds.ensureSenderConnected(); err != nil {
		return err
	}
	if err := rds.base.SendDatagram(d); err != nil {
		return err
	}

	// Update stats.
	rds.d.modInfo(func(i *Info) {
		i.PacketsSent++
		i.BytesSent += int64(len(d))
	})
	return nil
}

func (rds *remoteDynamicDatagramSender) MaxDatagramSize() int {
	return rds.lastDatagramSize
}

// We bother going through this process to get maximum reuse of a bound UDP
// DatagramSender. Since in the common case the base will not change, we can
// avoid the overhead of binding to a new port for each packet most of the time.
func (rds *remoteDynamicDatagramSender) ensureSenderConnected() error {
	addr, ok := rds.d.Addr().(*net.UDPAddr)
	if !ok {
		return errors.New("device address is not a *net.UDPAddr")
	}

	// Loop repeatedly until the address settles and we can return with a reader
	// lock.
	addrMatches := func() bool {
		return rds.base != nil &&
			(addr.IP.Equal(rds.baseAddr.IP) && addr.Port == rds.baseAddr.Port)
	}

	// (Common case) Do we have a base Sender, and does it match the address?
	if addrMatches() {
		return nil
	}

	// Close our current base, if we have one.
	if rds.base != nil {
		if err := rds.base.Close(); err != nil {
			return err
		}
	}

	w, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return err
	}

	rds.base = network.UDPDatagramSender(w)
	rds.baseAddr = addr
	rds.lastDatagramSize = rds.base.MaxDatagramSize()
	return nil
}

// remoteSender is an individual Sender instance that uses a shared dispatcher
// singleton to write.
type remoteSender struct {
	*packetDispatcher

	d *Remote
}

func (rs *remoteSender) Close() error { return rs.packetDispatcher.Release() }
