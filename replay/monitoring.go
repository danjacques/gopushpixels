// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package replay

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	recorderRecordingGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pixelproxy_recorder_recording",
		Help: "Count of active recorders recording.",
	})

	recorderErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pixelproxy_recorder_errors",
		Help: "Count of general recorder errors encountered.",
	}, []string{"type"})

	recorderEvents = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "pixelproxy_recorder_events",
		Help: "Count of recorded events.",
	})

	playerPlayingGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pixelproxy_player_playing",
		Help: "Count of active players replaying packets.",
	})

	playerPausedGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pixelproxy_player_paused",
		Help: "Incremented when the player is paused, decremented on resume.",
	})

	playerErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "pixelproxy_player_error_count",
		Help: "Count of player errors encountered during playback.",
	})

	playerDropped = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "pixelproxy_player_dropped_count",
		Help: "Count of dropped commands due to latency.",
	})

	playerCyclesGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pixelproxy_player_cycles",
		Help: "Count of discrete replay cycles in the current playback.",
	})

	playerSentBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "pixelproxy_player_sent_bytes",
		Help: "Count of bytes sent by the player.",
	})

	playerSentPackets = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "pixelproxy_player_sent_packets",
		Help: "Count of packets sent by the player.",
	})
)

// RegisterMonitoring registers all of this package's monitoring metrics.
func RegisterMonitoring(reg prometheus.Registerer) {
	reg.MustRegister(
		// Recorder
		recorderRecordingGauge,
		recorderErrors,
		recorderEvents,

		// Player
		playerPlayingGauge,
		playerPausedGauge,
		playerErrors,
		playerDropped,
		playerCyclesGauge,
		playerSentBytes,
		playerSentPackets,
	)
}
