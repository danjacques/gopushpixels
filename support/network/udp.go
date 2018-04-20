// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package network

import (
	"fmt"
	"net"

	"github.com/pkg/errors"
)

const (
	// MaxUDPSize is the largest UDP package size.
	MaxUDPSize = 65507
)

// ResolvedConn is a resolved address and its associated local interface
// information.
type ResolvedConn struct {
	// Interface is the network interface to connect on.
	Interface *net.Interface
	// Addr is the address to connect to.
	Addr *net.IPNet

	// Port is the port to connect to when dialing.
	//
	// If 0, a port will be chosen by the dialer.
	Port int

	// BufferSize, if >0, is the read/write buffer size to set on new connections.
	BufferSize int
}

// UDP4MulticastListenerConn generates a ResolvedConn configured for the
// default IPv4 multicast UDP address, DefaultIP4MulticastAddress.
//
// This is a suitable default for receiving multicast packets.
func UDP4MulticastListenerConn(port int) *ResolvedConn {
	return &ResolvedConn{
		Addr: &net.IPNet{
			IP: DefaultIP4MulticastAddress(),
		},
		Port: port,
	}
}

// UDP4MulticastTransmitterConn generates a ResolvedConn configured for the
// all-hosts IPv4 multicast UDP address, AllHostsMulticastIP4Address.
//
// This is a suitable default for broadcasting multicast packets.
func UDP4MulticastTransmitterConn(port int) *ResolvedConn {
	return &ResolvedConn{
		Addr: &net.IPNet{
			IP: DefaultIP4MulticastAddress(),
		},
		Port: port,
	}
}

func (rc *ResolvedConn) String() string {
	var base string
	switch {
	case rc.Interface != nil && rc.Addr != nil:
		base = fmt.Sprintf("%s on %s", rc.Addr, rc.Interface.Name)
	case rc.Interface != nil:
		base = rc.Interface.Name
	case rc.Addr != nil:
		base = rc.Addr.String()
	default:
		base = "unconfigured"
	}

	if rc.Port <= 0 {
		return base
	}
	return fmt.Sprintf("%s on port %d", base, rc.Port)
}

// ResolveInterface resolves the supplied interface name into a net.Interface.
// On success, rc's Interface will be populated with the result.
func (rc *ResolvedConn) ResolveInterface(name string) error {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}
	rc.Interface = iface
	return nil
}

// DialUDP4 creates a UDP connection configured with the configured parameters.
//
// If successful, the caller is responsible for closing the connection.
func (rc *ResolvedConn) DialUDP4() (*net.UDPConn, error) {
	addr := net.UDPAddr{
		IP:   rc.Addr.IP,
		Port: rc.Port,
	}

	conn, err := net.DialUDP("udp4", nil, &addr)
	if err != nil {
		return nil, err
	}

	if rc.BufferSize > 0 {
		if err := conn.SetWriteBuffer(rc.BufferSize); err != nil {
			_ = conn.Close()
			return nil, errors.Wrapf(err, "failed to set write buffer size to %d", rc.BufferSize)
		}
	}

	return conn, nil
}

// DatagramSender is a convenience method to generate a basic DatagramSender
// from the specified connection parameters.
func (rc *ResolvedConn) DatagramSender() (DatagramSender, error) {
	conn, err := rc.DialUDP4()
	if err != nil {
		return nil, err
	}
	return UDPDatagramSender(conn), nil
}

// ListenMulticastUDP4 creates a new listening net.UDPConn with the configured
// parameters.
//
// If Interface is nil, the system will choose an interface for you. As per
// net.ListenMulticastUDP, this is not recommended.
//
// If successful, the caller is responsible for closing the connection.
func (rc *ResolvedConn) ListenMulticastUDP4() (*net.UDPConn, error) {
	addr := net.UDPAddr{
		IP:   rc.Addr.IP,
		Port: rc.Port,
	}

	conn, err := net.ListenMulticastUDP("udp4", rc.Interface, &addr)
	if err != nil {
		return nil, err
	}

	if rc.BufferSize > 0 {
		if err := conn.SetReadBuffer(rc.BufferSize); err != nil {
			_ = conn.Close()
			return nil, errors.Wrapf(err, "failed to set read buffer size to %d", rc.BufferSize)
		}
	}

	return conn, nil
}

// AddressOptions is the set of options to use when resolving an address.
type AddressOptions struct {
	// If Interface is not empty, only addresses on the named interface will be
	// considered.
	Interface string

	// TargetAddr, if not nil, is the target address to use.
	//
	// At most one of TargetAddr or TargetAddress may be specified, with
	// TargetAddr taking priority if both are.
	TargetAddr *net.IPAddr
	// If TargetAddress is non-empty, Address will be resolved as a UDP address.
	//
	// At most one of TargetAddr or TargetAddress may be specified, with
	// TargetAddr taking priority if both are.
	TargetAddress string

	// If Multicast is true, only multicast addresses will be considered.
	// Otherwise, only non-multicast addresses will be considered.
	Multicast bool
}

// ResolveUDPAddress attempts to choose a network interface and address based on
// the supplied options.
func ResolveUDPAddress(opts AddressOptions) (*ResolvedConn, error) {
	var interfaces []net.Interface
	if opts.Interface != "" {
		iface, err := net.InterfaceByName(opts.Interface)
		if err != nil {
			return nil, errors.Wrapf(err, "could not find interface %q", opts.Interface)
		}

		interfaces = []net.Interface{*iface}
	} else {
		var err error
		if interfaces, err = net.Interfaces(); err != nil {
			return nil, errors.Wrap(err, "could not list network interfaces")
		}
	}

	targetIP := opts.TargetAddr
	if ta := opts.TargetAddress; ta != "" {
		var err error
		if targetIP, err = net.ResolveIPAddr("udp4", ta); err != nil {
			return nil, errors.Wrapf(err, "could not resolve 'udp4' address from %q", ta)
		}
	}

	// Choose a viable candidate interface.
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			// If we can't list addresses, skip this interface.
			continue
		}

		// When auto-selecting interface, ignore loopbacks.
		if targetIP == nil && isLoopbackInterface(addrs) {
			continue
		}

		if opts.Multicast {
			if addrs, err = iface.MulticastAddrs(); err != nil {
				continue
			}
		}

		// Iterate through each address and find a candidate.
		for _, addr := range addrs {
			// Only support IPv4 interfaces.
			ipNet := GetIPNet(addr)
			if ipNet == nil || ipNet.IP.To4() == nil {
				// Not an IPv4 interface.
				continue
			}

			// If we specify an address, make sure this contains it.
			if targetIP != nil && !ipNet.Contains(targetIP.IP) {
				continue
			}

			return &ResolvedConn{
				Interface: &iface,
				Addr:      ipNet,
			}, nil
		}
	}

	return nil, errors.New("could not identify an interface")
}

func isLoopbackInterface(addrs []net.Addr) bool {
	for _, addr := range addrs {
		ipNet := GetIPNet(addr)
		if ipNet == nil {
			continue
		}

		if ipNet.IP.IsLoopback() {
			return true
		}
	}

	return false
}

// GetIPNet returns the
func GetIPNet(addr net.Addr) *net.IPNet {
	switch t := addr.(type) {
	case *net.IPNet:
		return t
	case *net.IPAddr:
		return &net.IPNet{
			IP:   t.IP,
			Mask: t.IP.DefaultMask(),
		}
	case *net.UDPAddr:
		return &net.IPNet{
			IP:   t.IP,
			Mask: t.IP.DefaultMask(),
		}
	default:
		// Not an IP interface.
		return nil
	}
}
