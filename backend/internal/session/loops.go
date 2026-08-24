package session

import (
	"context"
	"io"
	"time"

	"github.com/quic-go/quic-go"

	"golive/internal/alf"
	"golive/internal/congestion/bbr"
	"golive/internal/logging"
	"golive/internal/metrics"
	"golive/internal/signal"
)

func (s *Session) loopBidi(ctx context.Context) {
	for {
		str, err := s.wt.AcceptStream(ctx)
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.readStream(ctx, str)
		}()
	}
}

func (s *Session) loopUni(ctx context.Context) {
	for {
		str, err := s.wt.AcceptUniStream(ctx)
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.readUni(ctx, str)
		}()
	}
}

func (s *Session) loopDatagram(ctx context.Context) {
	for {
		b, err := s.wt.ReceiveDatagram(ctx)
		if err != nil {
			return
		}
		s.ingest(b)
	}
}

func (s *Session) readStream(ctx context.Context, r io.Reader) {
	sr := alf.NewStreamReader(r, s.cfg.ALFMaxPayload+alf.HeaderSize)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		f, err := sr.ReadFrame()
		if err != nil {
			if err != io.EOF && ctx.Err() == nil {
				logging.L().Debug("bidi read end", "err", err, "sid", s.id)
			}
			return
		}
		raw, err := alf.Encode(f)
		if err != nil {
			continue
		}
		s.ingest(raw)
	}
}

func (s *Session) readUni(ctx context.Context, r io.Reader) {
	buf := make([]byte, s.cfg.MaxDatagram*4)
	// Uni video frames are sent as a single write of encoded ALF (possibly one fragment).
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		n, err := r.Read(buf)
		if n > 0 {
			s.ingest(append([]byte(nil), buf[:n]...))
		}
		if err != nil {
			return
		}
	}
}

func (s *Session) loopTick(ctx context.Context) {
	hz := s.cfg.MetricsHz
	if hz < 1 {
		hz = 10
	}
	t := time.NewTicker(time.Second / time.Duration(hz))
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.emitMetrics()
			_ = s.reasm.Sweep(time.Now())
		}
	}
}

func (s *Session) emitMetrics() {
	if s.conn == nil {
		return
	}
	st, cs, _, _, _ := s.col.Observe(s.conn)
	sample := bbrSample(st)
	out := s.ctrl.OnSample(sample)
	s.sched.ApplyBBR(out)

	snap := metrics.Snapshot{
		SessionID:       s.id,
		MinRTTMs:        metrics.DurationMs(st.MinRTT),
		LatestRTTMs:     metrics.DurationMs(st.LatestRTT),
		SmoothedRTTMs:   metrics.DurationMs(st.SmoothedRTT),
		MeanDeviationMs: metrics.DurationMs(st.MeanDeviation),
		BytesSent:       st.BytesSent,
		BytesReceived:   st.BytesReceived,
		PacketsSent:     st.PacketsSent,
		PacketsReceived: st.PacketsReceived,
		PacketsLost:     st.PacketsLost,
		BytesLost:       st.BytesLost,
		QUICVersion:     uint32(cs.Version),
		GSO:             cs.GSO,
		DatagramRemote:  cs.SupportsDatagrams.Remote,
		DatagramLocal:   cs.SupportsDatagrams.Local,
		MaxDatagram:     s.cfg.MaxDatagram,
		BBR:             out,
		Scheduler:       s.sched.Stats(),
		NetemLossUp:     s.netem.Uplink().LossPct,
		NetemLossDown:   s.netem.Downlink().LossPct,
	}
	s.col.Fill(&snap)
	s.pushSignal(signal.TypeMetrics, "", snap)
}

func bbrSample(st quic.ConnectionStats) bbr.Sample {
	return bbr.Sample{
		MinRTT:        int64(st.MinRTT),
		LatestRTT:     int64(st.LatestRTT),
		SmoothedRTT:   int64(st.SmoothedRTT),
		MeanDeviation: int64(st.MeanDeviation),
		BytesSent:     st.BytesSent,
		BytesReceived: st.BytesReceived,
		PacketsSent:   st.PacketsSent,
		PacketsLost:   st.PacketsLost,
		NowNs:         time.Now().UnixNano(),
	}
}
