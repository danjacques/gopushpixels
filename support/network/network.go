// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

// Package network contains generic network constants and utilities.
package network

import (
	"net"

	"github.com/pkg/errors"
)

// DefaultIP4MulticastAddress generates a default IPv4 multicast address.
func DefaultIP4MulticastAddress() net.IP { return net.IP{224, 0, 0, 0} }

// AllHostsMulticastIP4Address generates the IPv4 multicast address for "all
// hosts".
func AllHostsMulticastIP4Address() net.IP { return net.IP{255, 255, 255, 255} }

// ParseIP4Address parses the string, v, into an IPv4 address. If v failed to
// parse, or if v did not parse into an IPv4 address, an error will be returned.
func ParseIP4Address(v string) (net.IP, error) {
	ip := net.ParseIP(v)
	if ip == nil {
		return nil, errors.Errorf("could not parse IP address %q", v)
	}

	ip = ip.To4()
	if ip == nil {
		return nil, errors.Errorf("unable to get IPv4 address for %q", v)
	}

	return ip, nil
}
