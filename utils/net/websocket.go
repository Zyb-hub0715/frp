package net

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/fatedier/frp/utils/log"
	"golang.org/x/net/websocket"
)

type WebsocketListener struct {
	log.Logger
	server    *http.Server
	httpMutex *http.ServeMux
	connChan  chan *WebsocketConn
	closeFlag bool
}

func ListenWebsocket(bindAddr string, bindPort int) (l *WebsocketListener, err error) {
	l = &WebsocketListener{
		httpMutex: http.NewServeMux(),
		connChan:  make(chan *WebsocketConn),
		Logger:    log.NewPrefixLogger(""),
	}
	l.httpMutex.Handle("/", websocket.Handler(func(c *websocket.Conn) {
		conn := NewWebScoketConn(c)
		l.connChan <- conn
		conn.waitClose()
	}))
	l.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", bindAddr, bindPort),
		Handler: l.httpMutex,
	}
	ch := make(chan struct{})
	go func() {
		close(ch)
		err = l.server.ListenAndServe()
	}()
	<-ch
	<-time.After(time.Millisecond)
	return
}

func (p *WebsocketListener) Accept() (Conn, error) {
	c := <-p.connChan
	return c, nil
}

func (p *WebsocketListener) Close() error {
	if !p.closeFlag {
		p.closeFlag = true
		p.server.Close()
	}
	return nil
}

type WebsocketConn struct {
	net.Conn
	log.Logger
	closed int32
	wait   chan struct{}
}

func NewWebScoketConn(conn net.Conn) (c *WebsocketConn) {
	c = &WebsocketConn{
		Conn:   conn,
		Logger: log.NewPrefixLogger(""),
		wait:   make(chan struct{}),
	}
	return
}

func (p *WebsocketConn) Close() error {
	if atomic.LoadInt32(&p.closed) == 1 {
		return nil
	}
	close(p.wait)
	return p.Conn.Close()
}

func (p *WebsocketConn) waitClose() {
	<-p.wait
}

// ConnectWebsocketServer :
// addr: ws://domain:port
func ConnectWebsocketServer(addr string) (c Conn, err error) {
	addr = "ws://" + addr
	uri, err := url.Parse(addr)
	if err != nil {
		return
	}

	origin := "http://" + uri.Host
	cfg, err := websocket.NewConfig(addr, origin)
	if err != nil {
		return
	}
	cfg.Dialer = &net.Dialer{
		Timeout: time.Second * 10,
	}

	conn, err := websocket.DialConfig(cfg)
	if err != nil {
		return
	}
	c = NewWebScoketConn(conn)
	return
}
