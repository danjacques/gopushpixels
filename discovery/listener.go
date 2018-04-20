// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package discovery

import (
	"context"
	"io"
	"net"

	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/support/fmtutil"
	"github.com/danjacques/gopushpixels/support/logging"
	"github.com/danjacques/gopushpixels/support/network"

	"github.com/pkg/errors"
)

// listenerConnection models a *net.UDPConn.
//
// It is used internally to Listener for mocking.
type listenerConnection interface {
	io.Closer
	LocalAddr() net.Addr
	SetReadBuffer(int) error
	ReadFromUDP([]byte) (int, *net.UDPAddr, error)
}

// DefaultListenerConn returns a resolved connection configuration bound to
// the default device discovery port and multicast listener address.
func DefaultListenerConn() *network.ResolvedConn {
	return network.UDP4MulticastListenerConn(protocol.DiscoveryUDPPort)
}

type listenResult struct {
	packet []byte
	addr   net.Addr
	err    error
}

// Listener listens for PixelPusher broadcasts.
//
// When a user is finished with Listener, they should call Close to release its
// resources.
//
// Listener is not safe for concurrent use.
type Listener struct {
	// Logger, if not nil, is the Logger to log Listener status to.
	Logger logging.L

	// FilterFunc, if not nil, is called with a prospective set of DeviceHeaders.
	// If the function returns false, the device's discovery is ignored.
	FilterFunc func(dh *protocol.DiscoveryHeaders) bool

	conn   listenerConnection
	logger logging.L
	data   []byte

	requestC chan struct{}
	resultC  chan listenResult
}

// Close closes the Listener, interrupting any current operations and releasing
// its resources.
func (l *Listener) Close() error {
	if l.conn == nil {
		return nil
	}

	// Shut down goroutine.
	if l.requestC != nil {
		close(l.requestC)
	}

	// Close our listener.
	if err := l.conn.Close(); err != nil {
		return err
	}
	l.conn = nil
	return nil
}

// Start starts the Listener listening on the supplied connection, conn.
//
// Start will transfer ownership of conn to Listener regardless of success.
//
// Consider using network.ListenMulticastUDP4Helper to generate conn.
func (l *Listener) Start(conn *net.UDPConn) error { return l.startInternal(conn) }

func (l *Listener) startInternal(conn listenerConnection) error {
	if l.conn != nil {
		return errors.New("already connected")
	}

	// Resolve our Logger.
	l.logger = logging.Must(l.Logger)
	l.logger.Infof("Listening for discovery packets on %s...", conn.LocalAddr())

	// Set our read buffer size.
	if err := conn.SetReadBuffer(network.MaxUDPSize); err != nil {
		l.logger.Errorf("Failed to set read buffer size to %d: %s", network.MaxUDPSize, err)
		if cerr := conn.Close(); cerr != nil {
			l.logger.Errorf("Failed to close device on error: %s", cerr)
		}
		return err
	}

	l.conn = conn
	l.data = make([]byte, network.MaxUDPSize)
	l.requestC = make(chan struct{})
	l.resultC = make(chan listenResult, 1)

	// Start our listener goroutine.
	//
	// This approach is sane b/c this class is not safe for concurrent use, so
	// Accept calls will be serialized.
	go func() {
		// Wait for a request.
		for range l.requestC {
			// Block until the next multicast packet arrives.
			amt, addr, err := l.conn.ReadFromUDP(l.data)
			lr := listenResult{
				addr: addr,
				err:  err,
			}
			if err == nil {
				lr.packet = l.data[:amt]
			}

			// With a buffer size of 1, this should never default, but if it does
			// we'd rather drop the packet than miss a request.
			select {
			case l.resultC <- lr:
			default:
			}
		}
	}()

	return nil
}

// Accept blocks until a device broadcast is received.
//
// Listener must successfully Connect prior to using Accept.
func (l *Listener) Accept(c context.Context) (*protocol.DiscoveryHeaders, error) {
	// NOTE: the complexity of this function, notably using a proxy goroutine,
	// is due to the net package not directly supporting Context cancellation.
	if l.conn == nil {
		return nil, errors.New("the Listener is not active")
	}

	// Loop until we either hit an error or receive valid discovery headers.
	for {
		switch dh, err := l.acceptOnce(c); {
		case err != nil:
			return nil, err
		case dh == nil:
			// Filtered or invalid discovery packet.
		default:
			return dh, nil
		}
	}
}

// acceptOnce executes a single Accept call and retrieves a single discovery
// packet.
//
// An error will only be returned if an operation-level (not data-level) error
// is encountered.
//
// If the packet is filtered, or if the packet is not valid, it will log the
// status and return nil for both headers and error.
func (l *Listener) acceptOnce(c context.Context) (*protocol.DiscoveryHeaders, error) {
	// Clear any previous result in the queue.
	select {
	case <-l.resultC:
	case <-c.Done():
		// Context started in a cancelled state.
		return nil, c.Err()
	default:
	}

	// Make an Accept request.
	l.logger.Debug("Waiting for discovery packet...")
	l.requestC <- struct{}{}
	select {
	case lr := <-l.resultC:
		if lr.err != nil {
			return nil, lr.err
		}

		l.logger.Debugf("Discovery packet received (%d byte(s)):\n%s", len(lr.packet), fmtutil.Hex(lr.packet))

		// Parse the broadcast packet.
		dh, err := protocol.ParseDiscoveryHeaders(lr.packet)
		if err != nil {
			l.logger.Warnf("Failed to parse discovery packet; discarding: %s", err)
			return nil, nil
		}
		l.logger.Debugf("Received discovery broadcast: %s", dh)

		if dh.DeviceType != protocol.PixelPusherDeviceType {
			l.logger.Warnf("Received broadcast from non-PixelPusher (%s); discarding.", dh.DeviceType)
			return nil, nil
		}

		// Apply filter, if one is defined.
		if l.FilterFunc != nil && !l.FilterFunc(dh) {
			l.logger.Debugf("Device %s is explicitly filtered; ignoring.", dh.HardwareAddr())
			return nil, nil
		}

		// This is a valid PixelPusher discovery header!
		l.logger.Debugf("Received discovery for device address: %s", dh.HardwareAddr())
		return dh, nil

	case <-c.Done():
		return nil, c.Err()
	}
}
