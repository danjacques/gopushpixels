// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package proxy

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	proxyLeaseGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pixelproxy_proxy_leases",
		Help: "Number of current proxy leases.",
	})

	proxyReceivedPackets = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pixelproxy_proxy_received_packets",
		Help: "Count of packets received by a device proxy.",
	},
		[]string{"proxy_id", "base_id"})

	proxyReceivedBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pixelproxy_proxy_received_bytes",
		Help: "Count of bytes received by a device proxy.",
	},
		[]string{"proxy_id", "base_id"})

	proxySentPackets = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pixelproxy_proxy_sent_packets",
		Help: "Count of packets sent by a device proxy to its proxied device.",
	},
		[]string{"proxy_id", "base_id"})

	proxySentBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pixelproxy_proxy_sent_bytes",
		Help: "Count of bytes sent by a device proxy to its proxied device.",
	},
		[]string{"proxy_id", "base_id"})

	proxyRecvErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pixelproxy_proxy_recv_errors",
		Help: "Number of errors encountered while receiving packets.",
	},
		[]string{"proxy_id", "base_id"})

	proxyForwardErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pixelproxy_proxy_forward_errors",
		Help: "Number of errors encountered while forwarding packets to base devices.",
	},
		[]string{"proxy_id", "base_id"})
)

// RegisterMonitoring registers all of this package's monitoring metrics.
func RegisterMonitoring(reg prometheus.Registerer) {
	reg.MustRegister(
		// Manager metrics.
		proxyLeaseGauge,

		// Device metrics.
		proxyReceivedPackets,
		proxyReceivedBytes,
		proxySentPackets,
		proxySentBytes,
		proxyRecvErrors,
		proxyForwardErrors,
	)
}
