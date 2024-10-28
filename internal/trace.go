package internal

import (
	"crypto/tls"
	"net/http/httptrace"
	"net/textproto"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func BuildClientTrace() *httptrace.ClientTrace {
	var (
		getConnTime      time.Time
		dnsStartTime     time.Time
		connectTime      time.Time
		tlsHandshakeTime time.Time
	)
	return &httptrace.ClientTrace{
		GetConn: func(hostPort string) {
			log.Tracef("[GetConn] HostPort: %s", hostPort)
			getConnTime = time.Now()
		},

		GotConn: func(info httptrace.GotConnInfo) {
			log.Tracef("[GotConn] LocalAddr: %s", info.Conn.LocalAddr())
			log.Tracef("[GotConn] RemoteAddr: %s", info.Conn.RemoteAddr())
			log.Tracef("[GotConn] Reused: %v", info.Reused)
			log.Tracef("[GotConn] WasIdle: %#v", info.WasIdle)
			if info.WasIdle {
				log.Tracef("[GotConn] IdleTime: %#v", info.IdleTime)
			}
			log.Tracef("[GotConn] Duration: %s", time.Since(getConnTime).String())
		},

		PutIdleConn: func(err error) {
			log.Tracef("[PutIdeConn] Error: %v", err)
		},

		GotFirstResponseByte: func() {
			log.Tracef("[GotFirstResponseByte]")
		},

		Got100Continue: func() {
			log.Tracef("[Got100Continue]")
		},

		DNSStart: func(info httptrace.DNSStartInfo) {
			log.Tracef("[DNSStart] Host: %s", info.Host)
			dnsStartTime = time.Now()
		},

		DNSDone: func(info httptrace.DNSDoneInfo) {
			addrs := make([]string, 0, len(info.Addrs))
			for _, addr := range info.Addrs {
				addrs = append(addrs, addr.String())
			}
			log.Tracef("[DNSDone] Addrs: %s", strings.Join(addrs, ","))
			log.Tracef("[DNSDone] Coalesced: %v", info.Coalesced)
			log.Tracef("[DNSDone] Error: %v", info.Err)
			log.Tracef("[DNSDone] Duration: %s", time.Since(dnsStartTime).String())
		},

		ConnectStart: func(network, addr string) {
			log.Tracef("[ConnectStart] Network: %s", network)
			log.Tracef("[ConnectStart] Addr: %s", addr)
			connectTime = time.Now()
		},

		ConnectDone: func(network, addr string, err error) {
			log.Tracef("[ConnectDone] Network: %s", network)
			log.Tracef("[ConnectDone] Addr: %s", addr)
			log.Tracef("[ConnectDone] Error: %v", err)
			log.Tracef("[ConnectDone] Duration: %s", time.Since(connectTime).String())
		},

		WroteHeaders: func() {
			log.Tracef("[WroteHeaders]")
		},

		Wait100Continue: func() {
			log.Tracef("[Wait100Continue]")
		},

		WroteRequest: func(info httptrace.WroteRequestInfo) {
			log.Tracef("[WroteRequest] Error: %v", info.Err)
		},
		TLSHandshakeStart: func() {
			log.Tracef("[TLSHandshakeStart]")
			tlsHandshakeTime = time.Now()
		},
		TLSHandshakeDone: func(cs tls.ConnectionState, err error) {
			log.Tracef("[TLSHandshakeDone] Version: %d", cs.Version)
			log.Tracef("[TLSHandshakeDone] HandshakeComplete: %v", cs.HandshakeComplete)
			log.Tracef("[TLSHandshakeDone] DidResume: %v", cs.DidResume)
			log.Tracef("[TLSHandshakeDone] CipherSuite: %d", cs.CipherSuite)
			log.Tracef("[TLSHandshakeDone] NegotiatedProtocol: %s", cs.NegotiatedProtocol)
			log.Tracef("[TLSHandshakeDone] ServerName: %s", cs.ServerName)
			log.Tracef("[TLSHandshakeDone] Error: %v", err)
			log.Tracef("[TLSHandshakeDone] Duration: %s", time.Since(tlsHandshakeTime).String())
		},
		WroteHeaderField: func(key string, value []string) {
			log.Tracef("[WroteHeaderField] key: %s, value: %v", key, strings.Join(value, ","))
		},
		Got1xxResponse: func(code int, header textproto.MIMEHeader) error {
			log.Tracef("[Got1xxResponse] code: %d, header: %+v", code, header)
			return nil
		},
	}
}
