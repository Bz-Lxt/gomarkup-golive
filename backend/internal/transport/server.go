package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"

	"golive/internal/certs"
	"golive/internal/config"
	"golive/internal/logging"
	"golive/internal/netem"
	"golive/internal/room"
	"golive/internal/session"
)

type ctxKey int

const quicConnCtx ctxKey = 1

type Server struct {
	cfg    *config.Config
	bundle *certs.Bundle
	dual   *netem.Dual
	hub    *room.Hub
	reg    *session.Registry
	wt     *webtransport.Server
	udp    net.PacketConn
	parent context.Context

	connsMu sync.Mutex
	conns   map[string]*quic.Conn
}

func New(parent context.Context, cfg *config.Config, bundle *certs.Bundle, dual *netem.Dual, hub *room.Hub, reg *session.Registry) *Server {
	return &Server{cfg: cfg, bundle: bundle, dual: dual, hub: hub, reg: reg, parent: parent, conns: make(map[string]*quic.Conn)}
}

func (s *Server) Dual() *netem.Dual         { return s.dual }
func (s *Server) Registry() *session.Registry { return s.reg }
func (s *Server) Hub() *room.Hub            { return s.hub }

func (s *Server) ListenAndServe() error {
	udpAddr, err := net.ResolveUDPAddr("udp", s.cfg.UDPAddr)
	if err != nil {
		return fmt.Errorf("resolve udp: %w", err)
	}
	raw, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listen udp: %w", err)
	}
	wrapped := netem.Wrap(raw, s.dual)
	s.udp = wrapped

	tlsConf := http3.ConfigureTLSConfig(&tls.Config{
		Certificates: []tls.Certificate{s.bundle.TLSCertificate()},
		NextProtos:   []string{http3.NextProtoH3},
		MinVersion:   tls.VersionTLS13,
	})

	h3 := &http3.Server{
		Addr:      s.cfg.UDPAddr,
		TLSConfig: tlsConf,
		QUICConfig: &quic.Config{
			EnableDatagrams:                  true,
			EnableStreamResetPartialDelivery: true,
			MaxIdleTimeout:                   s.cfg.SessionIdle,
			KeepAlivePeriod:                  10 * time.Second,
			MaxIncomingStreams:               256,
			MaxIncomingUniStreams:            256,
			InitialStreamReceiveWindow:       2 << 20,
			MaxStreamReceiveWindow:           8 << 20,
			InitialConnectionReceiveWindow:   4 << 20,
			MaxConnectionReceiveWindow:       16 << 20,
		},
		ConnContext: func(ctx context.Context, conn *quic.Conn) context.Context {
			if conn != nil {
				s.remember(conn)
			}
			return context.WithValue(ctx, quicConnCtx, conn)
		},
	}
	webtransport.ConfigureHTTP3Server(h3)

	mux := http.NewServeMux()
	h3.Handler = mux
	s.wt = &webtransport.Server{
		H3:                   h3,
		ApplicationProtocols: []string{"golive"},
		CheckOrigin:          AllowOrigin(s.cfg.AllowedOrigins),
	}
	mux.HandleFunc("/webtransport", s.upgrade)

	logging.L().Info("webtransport listening", "addr", s.cfg.UDPAddr, "url", s.cfg.WebTransportURL())
	return s.wt.Serve(wrapped)
}

func (s *Server) upgrade(w http.ResponseWriter, r *http.Request) {
	sess, err := s.wt.Upgrade(w, r)
	if err != nil {
		logging.L().Warn("webtransport upgrade failed", "err", err, "origin", r.Header.Get("Origin"))
		return
	}
	var src session.ConnSource
	if c, ok := r.Context().Value(quicConnCtx).(*quic.Conn); ok && c != nil {
		src = c
	} else if c := s.lookup(r.RemoteAddr); c != nil {
		src = c
	}
	live := session.New(s.parent, s.cfg, sess, src, s.hub, s.dual)
	s.reg.Add(live)
	logging.L().Info("session accepted", "sid", live.ID(), "remote", r.RemoteAddr)
	go func() {
		defer s.reg.Remove(live.ID())
		live.Run()
	}()
}

func (s *Server) Close() error {
	if s.wt != nil {
		_ = s.wt.Close()
	}
	if s.udp != nil {
		return s.udp.Close()
	}
	return nil
}

func (s *Server) remember(conn *quic.Conn) {
	key := conn.RemoteAddr().String()
	s.connsMu.Lock()
	s.conns[key] = conn
	s.connsMu.Unlock()
	context.AfterFunc(conn.Context(), func() {
		s.connsMu.Lock()
		delete(s.conns, key)
		s.connsMu.Unlock()
	})
}

func (s *Server) lookup(remote string) *quic.Conn {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	if c, ok := s.conns[remote]; ok {
		return c
	}
	// last-resort: single live connection
	if len(s.conns) == 1 {
		for _, c := range s.conns {
			return c
		}
	}
	return nil
}

func ConnFromRequest(r *http.Request) *quic.Conn {
	c, _ := r.Context().Value(quicConnCtx).(*quic.Conn)
	return c
}
