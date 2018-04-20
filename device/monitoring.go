// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package device

import (
	"reflect"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	deviceOnlineGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "device_online",
		Help: "Count of currently-online devices.",
	},
		[]string{"type", "id"})

	devicePixelCountGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "device_pixel_count",
		Help: "Total number of pixels attached to a given device.",
	},
		[]string{"type", "id"})

	deviceStripCountGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "device_strip_count",
		Help: "Count of strips attached to a given device.",
	},
		[]string{"type", "id"})

	deviceWritePackets = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "device_write_packets",
		Help: "Count of packets written by a device.",
	},
		[]string{"type", "id"})

	deviceWriteBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "device_write_bytes",
		Help: "Count of bytes written by a remote device.",
	},
		[]string{"type", "id"})

	deviceWriteErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "device_write_errors",
		Help: "Count of errors encountered writing packets to a remote device.",
	},
		[]string{"type", "id"})
)

// RegisterMonitoring registers all of this package's monitoring metrics.
func RegisterMonitoring(reg prometheus.Registerer) {
	reg.MustRegister(
		deviceOnlineGauge,
		devicePixelCountGauge,
		deviceStripCountGauge,
		deviceWritePackets,
		deviceWriteBytes,
		deviceWriteErrors,
	)
}

// Monitoring is a thin wrapper around a D that logs monitoring information
// about that device.
type Monitoring struct {
	initOnce sync.Once
	labels   prometheus.Labels
}

// Update updates device metrics.
func (md *Monitoring) Update(d D) {
	md.initOnce.Do(func() {
		md.labels = monitoredDeviceLabels(d)
	})

	// If this device is Done, then clear all metrics.
	if IsDone(d) {
		deviceOnlineGauge.With(md.labels).Set(0)
		devicePixelCountGauge.With(md.labels).Set(0)
		deviceStripCountGauge.With(md.labels).Set(0)
		return
	}

	dh := d.DiscoveryHeaders()
	if dh == nil {
		return
	}
	deviceOnlineGauge.With(md.labels).Inc()
	devicePixelCountGauge.With(md.labels).Set(float64(dh.NumPixels()))
	deviceStripCountGauge.With(md.labels).Set(float64(dh.NumStrips()))
}

// MonitorSender wraps a Sender from d in a monitoring shim.
func MonitorSender(d D, s Sender) Sender {
	return &monitoredSender{
		Sender: s,
		labels: monitoredDeviceLabels(d),
	}
}

type monitoredSender struct {
	Sender
	labels prometheus.Labels
}

func (pw *monitoredSender) SendDatagram(d []byte) error {
	if err := pw.Sender.SendDatagram(d); err != nil {
		deviceWriteErrors.With(pw.labels).Inc()
		return err
	}

	deviceWritePackets.With(pw.labels).Inc()
	deviceWriteBytes.With(pw.labels).Add(float64(len(d)))
	return nil
}

func monitoredDeviceLabels(d D) prometheus.Labels {
	return prometheus.Labels{
		"type": reflect.TypeOf(d).String(),
		"id":   d.ID(),
	}
}
