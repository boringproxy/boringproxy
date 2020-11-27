// NOTE: The code in this file was mostly copied from this very helpful
// article:
// https://www.agwa.name/blog/post/writing_an_sni_proxy_in_go

package main

import (
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"time"
)

type readOnlyConn struct {
	reader io.Reader
}

func (conn readOnlyConn) Read(p []byte) (int, error)         { return conn.reader.Read(p) }
func (conn readOnlyConn) Write(p []byte) (int, error)        { return 0, io.ErrClosedPipe }
func (conn readOnlyConn) Close() error                       { return nil }
func (conn readOnlyConn) LocalAddr() net.Addr                { return nil }
func (conn readOnlyConn) RemoteAddr() net.Addr               { return nil }
func (conn readOnlyConn) SetDeadline(t time.Time) error      { return nil }
func (conn readOnlyConn) SetReadDeadline(t time.Time) error  { return nil }
func (conn readOnlyConn) SetWriteDeadline(t time.Time) error { return nil }

func peekClientHello(reader io.Reader) (*tls.ClientHelloInfo, io.Reader, error) {
	peekedBytes := new(bytes.Buffer)
	hello, err := readClientHello(io.TeeReader(reader, peekedBytes))
	if err != nil {
		return nil, nil, err
	}
	return hello, io.MultiReader(peekedBytes, reader), nil
}

func readClientHello(reader io.Reader) (*tls.ClientHelloInfo, error) {
	var hello *tls.ClientHelloInfo

	err := tls.Server(readOnlyConn{reader: reader}, &tls.Config{
		GetConfigForClient: func(argHello *tls.ClientHelloInfo) (*tls.Config, error) {
			hello = new(tls.ClientHelloInfo)
			*hello = *argHello
			return nil, nil
		},
	}).Handshake()

	if hello == nil {
		return nil, err
	}

	return hello, nil
}

type PassthroughListener struct {
	ch chan net.Conn
}

func NewPassthroughListener() *PassthroughListener {
	return &PassthroughListener{
		ch: make(chan net.Conn),
	}
}
func (f *PassthroughListener) Accept() (net.Conn, error) {
	return <-f.ch, nil
}
func (f *PassthroughListener) Close() error {
	return nil
}
func (f *PassthroughListener) Addr() net.Addr {
	return nil
}
func (f *PassthroughListener) PassConn(conn net.Conn) {
	f.ch <- conn
}

// This type creates a new net.Conn that's the same as an old one, except a new
// reader is provided. So it proxies every method except Read. I'm sure there's
// a cleaner way to do this...
type ProxyConn struct {
	conn   net.Conn
	reader io.Reader
}

func NewProxyConn(conn net.Conn, reader io.Reader) *ProxyConn {
	return &ProxyConn{
		conn,
		reader,
	}
}
func (c ProxyConn) CloseWrite() error           { return c.conn.(*net.TCPConn).CloseWrite() }
func (c ProxyConn) Read(p []byte) (int, error)  { return c.reader.Read(p) }
func (c ProxyConn) Write(p []byte) (int, error) { return c.conn.Write(p) }

// TODO: is this safe? Will it actually close properly?
func (c ProxyConn) Close() error                       { return c.conn.Close() }
func (c ProxyConn) LocalAddr() net.Addr                { return c.conn.LocalAddr() }
func (c ProxyConn) RemoteAddr() net.Addr               { return c.conn.RemoteAddr() }
func (c ProxyConn) SetDeadline(t time.Time) error      { return c.conn.SetDeadline(t) }
func (c ProxyConn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c ProxyConn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }
