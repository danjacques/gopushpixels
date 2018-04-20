// Copyright 2018 Dan Jacques. All rights reserved.
// Use of this source code is governed under the MIT License
// that can be found in the LICENSE file.

package replay

import (
	"context"
	"io"
	"time"

	"github.com/danjacques/gopushpixels/device"
	"github.com/danjacques/gopushpixels/protocol"
	"github.com/danjacques/gopushpixels/replay/streamfile"
	"github.com/danjacques/gopushpixels/support/logging"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
)

// PlaybackLeaser is a mechanism that a Player can use to claim a playback
// lease, blocking (cooperatively) other facilities from performing their own
// playback and sending interfering signals.
//
// PlaybackLeaser is cooperative; it is up to all participating playback
// mechanisms to sort out who gets the playback lease.
type PlaybackLeaser interface {
	// AcquirePlaybackLease attempts to take out a playback lease.
	AcquirePlaybackLease()
	// ReleasePLaybackLease releases any held playback lease.
	//
	// No no lease is held, ReleasePlaybackLease will do nothing.
	ReleasePlaybackLease()
}

// Player plays a stream file back to a sink.
//
// A Player is not safe for concurrent use. Its exported fields must not be
// changed after playback has begun.
type Player struct {
	// SendPacket receives all playback packets. It must not be nil.
	//
	// SendPacket calls will be made synchronously.
	SendPacket func(ord device.Ordinal, id string, pkt *protocol.Packet) error

	// PlaybackLeaser, if not nil, will be used to acquire and release leases
	// depending on the Player's playback status.
	PlaybackLeaser PlaybackLeaser

	// Logger is the logger instance to use. If nil, no logging will be
	// performed.
	Logger logging.L

	// MaxLagAge is the maximum amount of time in the past that we will allow a
	// packet to be scheduled. If the packet is older than this, we will drop it
	// and resume the stream once we hit future packets.
	MaxLagAge time.Duration

	ctx        context.Context
	cancelFunc context.CancelFunc

	playback *playerPlayback
}

// Play clears any currently playing file and begins playback of sr.
//
// Play takes ownership of sr, and Will close it when stopped.
func (p *Player) Play(c context.Context, sr *streamfile.EventStreamReader) {
	// Stop any currently-playing file.
	p.Stop()

	// We will cancel the Context ourselves on Stop. Retain this Context.
	p.ctx, p.cancelFunc = context.WithCancel(c)
	c = p.ctx

	// Initialize player resources.
	p.playback = &playerPlayback{
		player:         p,
		sr:             sr,
		logger:         logging.Must(p.Logger),
		leaser:         p.PlaybackLeaser,
		noRouteDevices: make(map[int64]int64),
		commandC:       make(chan *playerCommand),
		immediateC:     make(chan time.Time),
		finishedC:      make(chan struct{}),
	}
	close(p.playback.immediateC) // Always closed.

	// Must not have a nil leaser.
	if p.playback.leaser == nil {
		p.playback.leaser = noopPlaybackLeaser{}
	}

	// Start the player goroutine.
	//
	// We will play until the Context is cancelled.
	go func() {
		defer func() {
		}()
		p.playback.playUntilStopped(c)
	}()
}

// Status returns the current player status.
//
// If the player is not playing, Status will return nil.
func (p *Player) Status() *PlayerStatus {
	if p.playback == nil {
		return nil
	}

	statusC := make(chan *PlayerStatus, 1)
	p.playback.sendCommand(&playerCommand{status: statusC})

	select {
	case <-p.ctx.Done():
		return nil
	case status := <-statusC:
		return status
	}
}

// Pause pauses a current play operation. If nothing is playiung, or if the
// playback is already paused, Pause will do nothing.
func (p *Player) Pause() {
	p.playback.sendCommand(&playerCommand{pause: true})
}

// Resume resumes a paused file. If nothing is playing, or if a file is not
// paused, Resume will do nothing.
func (p *Player) Resume() {
	p.playback.sendCommand(&playerCommand{resume: true})
}

// Stop stops player playback and clears player resources.
func (p *Player) Stop() {
	if p.playback == nil {
		return
	}

	p.cancelFunc()
	<-p.playback.finishedC

	// Clean up any remaining resources.
	close(p.playback.commandC)
	p.playback = nil
}

// PlayerStatus describes the player's current status.
type PlayerStatus struct {
	Path           string
	Rounds         int64
	Position       time.Duration
	Duration       time.Duration
	TotalPlaytime  time.Duration
	Paused         bool
	NoRouteDevices []*PlayerStatusNoRouteDeviceEntry
}

// PlayerStatusNoRouteDeviceEntry is a PlayetStatus entry in the NoRouteDevices.
type PlayerStatusNoRouteDeviceEntry struct {
	ID      string
	Ordinal device.Ordinal
	Count   int64
}

type noopPlaybackLeaser struct{}

func (l noopPlaybackLeaser) AcquirePlaybackLease() {}
func (l noopPlaybackLeaser) ReleasePlaybackLease() {}

// playerCommand is a command sent to the player's goroutine.
type playerCommand struct {
	pause  bool
	resume bool

	status chan<- *PlayerStatus
}

// errRoundFinished is a sentinel error returned by waitFroNextCommandOrEvent
// to indicate that nothing went wrong, but the round should terminate.
var errRoundFinished = errors.New("round finished")

type playerPlayback struct {
	player *Player

	sr     *streamfile.EventStreamReader
	logger logging.L

	// leaser is the resolved PlaybackLeaser. It will never be nil.
	leaser PlaybackLeaser

	// noRouteDevices tracks the number of errors that have occurred because a
	// packet was destined for a device (metadata index) with no route.
	noRouteDevices map[int64]int64

	commandC   chan *playerCommand
	immediateC chan time.Time // Looks like a timer channel.
	finishedC  chan struct{}

	// playerStartTime is the time when the player started its first round.
	playerStartTime time.Time

	// roundCount is a count of the the number of playback rounds that have been
	// started.
	roundCount int64

	// startTime is the time when the Player began playing.
	startTime time.Time
	// realtimeOffset is the amount of time that we spent paused. This allows us
	// to offset the packet time and determine our stream position even if we
	// pause.
	realtimeOffset time.Duration
	// timer is the Timer used to sleep in between events.
	timer *time.Timer
}

// sendCommand issues a command to the playerPlayback.
//
// For convenience, if pp is nil, the command will be dropped. This helps avoid
// the need to check for nil for every command issuance point.
func (pp *playerPlayback) sendCommand(cmd *playerCommand) {
	if pp == nil {
		return
	}

	// Send the command.
	select {
	case pp.commandC <- cmd:
	case <-pp.finishedC:
	}
}

// playUntilStopped is run in its own goroutine. It plays the contents of the
// event stream repeatedly until its Context is cancelled.
func (pp *playerPlayback) playUntilStopped(c context.Context) {
	// When finished, close our stream.
	defer func() {
		// Stop our timer.
		if pp.timer != nil {
			pp.timer.Stop()
		}

		if err := pp.sr.Close(); err != nil {
			pp.logger.Warnf("Failed to close stream for %q: %s", pp.sr.Path(), err)
		}

		// Signal that we've finished.
		close(pp.finishedC)

		// Release our playback lease, if we have one.
		pp.leaser.ReleasePlaybackLease()

		// Consume any superfluous commands.
		//
		// Since finishedC is closed, no new commands will be sent.
		for range pp.commandC {
		}
	}()

	// Set our playing metric. Clear them when we're finished.
	playerPlayingGauge.Set(1)
	playerPausedGauge.Set(0)
	playerCyclesGauge.Set(0)
	defer func() {
		playerPlayingGauge.Set(0)
		playerPausedGauge.Set(0)
		playerCyclesGauge.Set(0)
	}()

	// Acquire a playback lease.
	pp.leaser.AcquirePlaybackLease()

	// Playback until finished.
	pp.playerStartTime = time.Now()

	for {
		// Check to see if the Context is cancelled yet.
		select {
		case <-c.Done():
			return
		default:
		}

		// Begin the next round.
		playerCyclesGauge.Inc()
		pp.logger.Infof("Starting player round #%d for %q...", pp.roundCount, pp.sr.Path())
		pp.roundCount++

		// Reset our stream to its starting position.
		if err := pp.sr.Reset(); err != nil {
			pp.logger.Errorf("Failed to reset stream: %s", err)
		}

		if err := pp.playRound(c); err != nil && errors.Cause(err) != context.Canceled {
			pp.logger.Warnf("Error during playback: %s", err)
			playerErrors.Inc()
			return
		}
	}
}

func (pp *playerPlayback) playRound(c context.Context) error {
	// Initialize round data.
	pp.startTime = time.Now()
	pp.realtimeOffset = 0

	// Loop until we've exhausted our stream.
	for {
		// Read the next event from the stream.
		e, err := pp.sr.ReadEvent()
		if err != nil {
			if err == io.EOF {
				pp.logger.Debugf("Hit EOF reading events.")
				return nil
			}

			pp.logger.Errorf("Could not read next event: %s", err)
			return err
		}

		// Wait for our next event offset to pass.
		//
		// The returned delta is the difference between when the event
		delta, err := pp.waitForNextCommandOrEvent(c, pp.sr.Position())
		if err != nil {
			// Handle sentinel error.
			if err == errRoundFinished {
				// Don't report this as an error up the chain, but terminate this round.
				err = nil
			}
			return err
		}

		// If our packet is scheduled in the past, consider dropping it if it's too
		// far behind.
		if effectiveDelta := delta + pp.player.MaxLagAge; effectiveDelta < 0 {
			pp.logger.Infof("Packet (offset %s) is beyond maximum lag age by %s; discarding. %v",
				delta, effectiveDelta, e.GetPacket())
			playerDropped.Inc()
			continue
		}

		// Send this packet.
		if pkt := e.GetPacket(); pkt != nil {
			packetDevice := pp.sr.ResolveDeviceForIndex(pkt.Device)
			if packetDevice == nil {
				pp.logger.Warnf("File references unknown device index #%d", pkt.Device)
				playerErrors.Inc()
				continue
			}

			epkt, err := pkt.Decode(packetDevice)
			if err != nil {
				pp.logger.Warnf("Could not decode command %s: %s", pkt, err)
				continue
			}

			size := proto.Size(pkt)
			ordinal := packetDevice.DeviceOrdinal()
			pp.logger.Debugf("Sending packet (size=%d) to device %s / %q.", size, ordinal, packetDevice.Id)
			switch err := pp.player.SendPacket(ordinal, packetDevice.Id, epkt); errors.Cause(err) {
			case nil:
				// Success! If this device was in a no-route list, purge it.
				delete(pp.noRouteDevices, pkt.Device)

			case device.ErrNoRoute:
				if _, ok := pp.noRouteDevices[pkt.Device]; !ok {
					pp.logger.Warnf("Could not route packet to device %q (no route).", packetDevice)
				}
				pp.noRouteDevices[pkt.Device]++

			default:
				pp.logger.Warnf("Could not route packet to device %q: %s", packetDevice.Id, err)
				playerErrors.Inc()
				continue
			}

			playerSentPackets.Inc()
			playerSentBytes.Add(float64(size))
		}
	}
}

// waitForNextCommandOrEvent blocks until the next command/event is ready.
//
// This is the main control point for the player.
//
// Pause is implemented by toggling a boolean which, if true, will remove the
// offset-based timer from the list of unblockers, causing the loop to block
// pending a new command (potentially play) or cancellation.
func (pp *playerPlayback) waitForNextCommandOrEvent(c context.Context, offset time.Duration) (time.Duration, error) {
	// Ensure that, when we exit, we always re-acquire our playback lease.
	defer pp.leaser.AcquirePlaybackLease()

	// If pausedStart is not zero, then we are paused.
	//
	// When we exit this loop, we are definitely not paused, so clear the paused
	// state then.
	pausedStart := time.Time{}
	defer playerPausedGauge.Set(0)

	// Stupid timer stuff that we have to do in order to reuse a timer.
	timerRunning := false
	resetTimer := func() {
		// If the timer was running, and it has now stopped, consume the signal
		// on its channel.
		if timerRunning && !pp.timer.Stop() {
			<-pp.timer.C
		}
		timerRunning = false
	}

	// Handle a player command, updating our "wait" state.
	processCommand := func(cmd *playerCommand) error {
		switch {
		case cmd.pause:
			pausedStart = time.Now()

			// When paused, we release our playback lease. We will re-acquire it
			// later on resume.
			pp.logger.Info("Player is paused. Releasing playback lease...")
			pp.leaser.ReleasePlaybackLease()
			playerPausedGauge.Set(1)

		case cmd.resume:
			if !pausedStart.IsZero() {
				pp.logger.Info("Player is resuming. Acquiring playback lease...")
				pp.leaser.AcquirePlaybackLease()

				// Add the amount of time that we were paused to our realtime offset.
				pp.realtimeOffset += time.Now().Sub(pausedStart)

				// Mark that we're no longer paused.
				pausedStart = time.Time{}
				playerPausedGauge.Set(0)
			}

		case cmd.status != nil:
			status := pp.getStatus()
			status.Paused = !pausedStart.IsZero()

			// Calculate the total playtime. This can be a little tricky, since
			// we don't want to count time that we've been paused.
			//
			// We can factor in previous pause rounds by subtracting realtimeOffset.
			// We can factor in the current pause round (if applicable) by subtracting
			// it explicitly.
			now := time.Now()
			totalPlaytime := now.Sub(pp.playerStartTime) - pp.realtimeOffset
			if !pausedStart.IsZero() {
				totalPlaytime -= now.Sub(pausedStart)
			}
			status.TotalPlaytime = totalPlaytime

			cmd.status <- status
		}
		return nil
	}

	// Select until we've reached our offset or encounter an error.
	for {
		// Quick pass to see if there's a command ready.
		select {
		case cmd := <-pp.commandC:
			if err := processCommand(cmd); err != nil {
				return 0, err
			}
			continue
		case <-c.Done():
			return 0, c.Err()
		default:
			// No pending commands.
		}

		streamNow := time.Now().Add(-pp.realtimeOffset)
		nextEventTime := pp.startTime.Add(offset)

		var timerC <-chan time.Time
		delta := time.Duration(0)
		switch {
		case !pausedStart.IsZero():
			// If we're paused, then we will never trigger on a timer.

		case !nextEventTime.After(streamNow):
			// The next event is now or in the past, so we can immediately trigger.
			timerC = pp.immediateC
			delta = streamNow.Sub(nextEventTime)

		default:
			// The next event is in the future. Initialize/start our timer.
			sleepDelta := nextEventTime.Sub(streamNow)
			pp.logger.Debugf("Sleeping %s until next packet @%s (d=%s)", sleepDelta, nextEventTime, offset)
			if pp.timer == nil {
				pp.timer = time.NewTimer(sleepDelta)
			} else {
				pp.timer.Reset(sleepDelta)
			}
			timerC = pp.timer.C
			timerRunning = true

			// Leave delta at "0". If the timer expires, this will cause us to report
			// that the event happened exactly when we wanted it to, smoothing over
			// noise from function execution and timer imperfection.
		}

		select {
		case cmd := <-pp.commandC:
			resetTimer()

			// We've received a command.
			if err := processCommand(cmd); err != nil {
				return 0, err
			}

		case <-c.Done():
			resetTimer()

			// We've finished our playback, or have been cancelled.
			return 0, c.Err()

		case _, ok := <-timerC:
			// Our timer has expired, indicating that we've hit our next offset.
			//
			// Note that if !ok, this is our immediateC optimization/hack happening,
			// not the actual timer expiring.
			if ok {
				timerRunning = false
			}
			resetTimer()

			// Return nil, indicating that we're ready for the next event.
			//
			// Return a delta of 0, indicating that the event is happening when
			// scheduled.
			return delta, nil
		}
	}
}

func (pp *playerPlayback) getStatus() *PlayerStatus {
	ps := PlayerStatus{
		Path:     pp.sr.Path(),
		Rounds:   pp.roundCount,
		Position: pp.sr.Position(),
		Duration: pp.sr.Duration(),
	}

	if len(pp.noRouteDevices) > 0 {
		ps.NoRouteDevices = make([]*PlayerStatusNoRouteDeviceEntry, 0, len(pp.noRouteDevices))
		for didx, count := range pp.noRouteDevices {
			d := pp.sr.ResolveDeviceForIndex(didx)
			if d != nil {
				ps.NoRouteDevices = append(ps.NoRouteDevices, &PlayerStatusNoRouteDeviceEntry{
					ID:      d.Id,
					Ordinal: d.DeviceOrdinal(),
					Count:   count,
				})
			}
		}
	}

	return &ps
}
