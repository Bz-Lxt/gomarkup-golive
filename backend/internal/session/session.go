package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/webtransport-go"

	"golive/internal/alf"
	"golive/internal/clock"
	"golive/internal/config"
	"golive/internal/congestion/bbr"
	"golive/internal/logging"
	"golive/internal/metrics"
	"golive/internal/netem"
	"golive/internal/room"
	"golive/internal/scheduler"
	"golive/internal/signal"
)

type ConnSource interface {
	ConnectionStats() quic.ConnectionStats
	ConnectionState() quic.ConnectionState
}

type Session struct {
	id     string
	cfg    *config.Config
	wt     *webtransport.Session
	conn   ConnSource
	room   *room.Room
	hub    *room.Hub
	roomID string
	mode   string
	netem  *netem.Dual

	ctrl  *bbr.Controller
	sched *scheduler.Scheduler
	col   *metrics.Collector
	reasm *alf.Reassembler
	files *fileSink
	mono  *clock.Mono

	seq   atomic.Uint64
	alive atomic.Bool
	last  atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	sigWriteMu sync.Mutex
	sigStream  *webtransport.Stream
}

type Info struct {
	ID         string `json:"id"`
	Room       string `json:"room"`
	Mode       string `json:"mode"`
	Remote     string `json:"remote"`
	Started    string `json:"started"`
	LastActive string `json:"last_active"`
}

func New(parent context.Context, cfg *config.Config, wt *webtransport.Session, conn ConnSource, hub *room.Hub, dual *netem.Dual) *Session {
	id := newID()
	ctx, cancel := context.WithCancel(parent)
	s := &Session{
		id:     id,
		cfg:    cfg,
		wt:     wt,
		conn:   conn,
		hub:    hub,
		roomID: "default",
		mode:   "echo",
		netem:  dual,
		ctrl:   bbr.New(cfg.BBR),
		col:    metrics.NewCollector(),
		reasm:  alf.NewReassembler(cfg.ReassemblyTTL, cfg.ALFMaxPayload),
		files:  newFileSink(),
		mono:   clock.NewMono(),
		ctx:    ctx,
		cancel: cancel,
	}
	s.alive.Store(true)
	s.touch()
	s.sched = scheduler.New(*cfg, s.ctrl, s.dispatchOut)
	return s
}

func (s *Session) ID() string { return s.id }

func (s *Session) Enqueue(it scheduler.Item) bool {
	return s.sched.Enqueue(it)
}

func (s *Session) LastActive() time.Time {
	return time.UnixMilli(s.last.Load())
}

func (s *Session) Info() Info {
	return Info{
		ID: s.id, Room: s.roomID, Mode: s.mode,
		Remote:     addrString(s.wt.RemoteAddr()),
		Started:    clock.Format(clock.Now()),
		LastActive: clock.Format(s.LastActive()),
	}
}

func (s *Session) Run() {
	s.room = s.hub.Get(s.roomID)
	s.room.Join(s)
	s.wg.Add(5)
	go s.guard("sched", s.sched.Run)
	go s.guard("bidi", s.loopBidi)
	go s.guard("uni", s.loopUni)
	go s.guard("dgram", s.loopDatagram)
	go s.guard("tick", s.loopTick)
	s.wg.Wait()
}

func (s *Session) guard(name string, fn func(context.Context)) {
	defer s.wg.Done()
	defer func() {
		if rec := recover(); rec != nil {
			logging.L().Error("session loop panic", "loop", name, "panic", rec, "sid", s.id)
		}
	}()
	fn(s.ctx)
}

func (s *Session) Close(reason string) {
	if !s.alive.CompareAndSwap(true, false) {
		return
	}
	logging.L().Info("session closing", "sid", s.id, "reason", reason)
	s.cancel()
	s.sched.Close()
	if s.room != nil {
		s.hub.Leave(s.roomID, s.id)
	}
	_ = s.wt.CloseWithError(0, reason)
}

func (s *Session) touch() {
	s.last.Store(clock.UnixMilliBeijing())
}

func (s *Session) nextSeq() uint64 { return s.seq.Add(1) }

func (s *Session) dispatchOut(it scheduler.Item) error {
	switch alf.DefaultKind(it.Channel) {
	case alf.KindDatagram:
		return s.wt.SendDatagram(it.Payload)
	case alf.KindUni:
		str, err := s.wt.OpenUniStreamSync(s.ctx)
		if err != nil {
			return err
		}
		_, err = str.Write(it.Payload)
		_ = str.Close()
		return err
	default:
		s.sigWriteMu.Lock()
		defer s.sigWriteMu.Unlock()
		if s.sigStream == nil {
			str, err := s.wt.OpenStreamSync(s.ctx)
			if err != nil {
				return err
			}
			s.sigStream = str
		}
		return alf.NewStreamWriter(s.sigStream).WriteFrame(mustDecodeOrRaw(it))
	}
}

func mustDecodeOrRaw(it scheduler.Item) alf.Frame {
	if f, err := alf.Decode(it.Payload); err == nil {
		return f
	}
	return alf.Frame{
		Channel: it.Channel, Kind: alf.DefaultKind(it.Channel),
		Priority: it.Priority, Seq: it.Seq, PTS: it.PTS,
		Flags: alf.FlagEndOfMessage, FragIdx: 0, FragTotal: 1,
		Payload: it.Payload,
	}
}

func (s *Session) handleFrame(f alf.Frame, raw []byte) {
	s.touch()
	switch f.Channel {
	case alf.ChannelSignal:
		s.handleSignal(f)
	case alf.ChannelFile:
		if err := s.files.Write(f.Payload); err != nil {
			logging.L().Warn("file write", "err", err, "sid", s.id)
		}
		if f.EndOfMessage() {
			ack := s.files.Finish()
			s.pushSignal(signal.TypeFileAck, "", ack)
		}
	default:
		s.col.NoteFrame(f.Channel.String())
		item := room.MediaItem(f.Channel, f.Seq, f.PTS, raw)
		item.OnDrop = func(reason string) {
			s.col.NoteDrop(f.Channel.String())
			logging.L().Debug("media drop", "ch", f.Channel.String(), "reason", reason)
		}
		if s.mode == "room" && s.room != nil {
			s.room.Fanout(s.id, item)
			return
		}
		_ = s.room.Echo(s.id, item)
	}
}

func (s *Session) handleSignal(f alf.Frame) {
	env, err := signal.Decode(f.Payload)
	if err != nil {
		logging.L().Warn("bad signal", "err", err, "sid", s.id)
		return
	}
	switch env.Type {
	case signal.TypeHello:
		var h signal.Hello
		_ = json.Unmarshal(env.Payload, &h)
		if h.Room != "" && h.Room != s.roomID {
			s.hub.Leave(s.roomID, s.id)
			s.roomID = h.Room
			s.room = s.hub.Get(s.roomID)
			s.room.Join(s)
		}
		if h.Mode == "room" || h.Mode == "echo" {
			s.mode = h.Mode
		}
		s.pushSignal(signal.TypeWelcome, env.ID, signal.Welcome{
			SessionID: s.id, Room: s.roomID, Mode: s.mode,
			WTURL: s.cfg.WebTransportURL(), MaxDatagram: s.cfg.MaxDatagram,
			Channels:   []string{"signal", "audio", "cursor", "video", "file"},
			BBRLayer:   "application-scheduler",
			ServerTime: clock.Format(clock.Now()),
		})
	case signal.TypeNetemSet:
		var req signal.NetemSet
		if err := json.Unmarshal(env.Payload, &req); err != nil {
			s.pushError(env.ID, "bad_netem", err.Error())
			return
		}
		if req.Preset != "" {
			if err := s.netem.ApplyPreset(req.Preset); err != nil {
				s.pushError(env.ID, "bad_preset", err.Error())
				return
			}
		} else if req.Uplink != nil && req.Downlink != nil {
			if err := req.Uplink.Validate(); err != nil {
				s.pushError(env.ID, "bad_uplink", err.Error())
				return
			}
			if err := req.Downlink.Validate(); err != nil {
				s.pushError(env.ID, "bad_downlink", err.Error())
				return
			}
			s.netem.Apply(*req.Uplink, *req.Downlink)
		}
		s.pushSignal(signal.TypeNetemOK, env.ID, s.netem.Snapshot())
	case signal.TypeBBRSet:
		var req signal.BBRSet
		_ = json.Unmarshal(env.Payload, &req)
		s.ctrl.SetEnabled(req.Enabled)
		s.pushSignal(signal.TypeBBROk, env.ID, map[string]any{"enabled": req.Enabled, "layer": "application-scheduler"})
	case signal.TypePing:
		var p signal.Ping
		_ = json.Unmarshal(env.Payload, &p)
		s.pushSignal(signal.TypePong, env.ID, signal.Pong{
			ClientTs: p.ClientTs, ServerTs: clock.UnixMilliBeijing(), Seq: p.Seq,
		})
	case signal.TypeFileBegin:
		var m signal.FileBegin
		if err := json.Unmarshal(env.Payload, &m); err != nil {
			s.pushError(env.ID, "bad_file", err.Error())
			return
		}
		if err := s.files.Begin(m); err != nil {
			s.pushError(env.ID, "file_begin", err.Error())
		}
	case signal.TypeFileDone:
		ack := s.files.Finish()
		s.pushSignal(signal.TypeFileAck, env.ID, ack)
	default:
		s.pushError(env.ID, "unknown_type", env.Type)
	}
}

func (s *Session) pushSignal(typ, id string, payload any) {
	f, err := signal.FrameFromJSON(alf.ChannelSignal, s.nextSeq(), s.mono.Tick(), typ, id, payload)
	if err != nil {
		logging.L().Warn("signal encode", "err", err)
		return
	}
	raw, err := alf.Encode(f)
	if err != nil {
		return
	}
	s.sched.Enqueue(scheduler.Item{
		Channel: alf.ChannelSignal, Priority: alf.PrioSignal,
		Seq: f.Seq, PTS: f.PTS, Payload: raw, Reliable: true,
	})
}

func (s *Session) pushError(id, code, msg string) {
	s.pushSignal(signal.TypeError, id, signal.ErrorBody{Code: code, Message: msg})
}

func (s *Session) ingest(raw []byte) {
	f, err := alf.Decode(raw)
	if err != nil {
		logging.L().Debug("alf decode", "err", err, "sid", s.id)
		return
	}
	done, ok, err := s.reasm.Push(f, time.Now())
	if err != nil {
		logging.L().Debug("reassemble", "err", err)
		return
	}
	if !ok {
		return
	}
	enc, err := alf.Encode(done)
	if err != nil {
		s.handleFrame(done, raw)
		return
	}
	s.handleFrame(done, enc)
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func addrString(a net.Addr) string {
	if a == nil {
		return ""
	}
	return a.String()
}
