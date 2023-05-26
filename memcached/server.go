// Package memcached provides an interface for building your
// own memcached ascii protocol servers.
package memcached

import (
	"bufio"
	"bytes"
	"net"
	"strconv"
)

const VERSION = "0.0.0"

var (
	crlf    = []byte("\r\n")
	noreply = []byte("noreply")
)

type Server struct {
	Addr    string
	Getter  Getter
	Setter  Setter
	Deleter Deleter
	Stats   Stats
}

type StorageCmd struct {
	Key     string
	Flags   int
	Exptime int64
	Length  int
	Noreply bool
}

func (s *Server) newConn(rwc net.Conn) *conn {
	return &conn{
		server: s,
		conn:   rwc,
		rwc:    bufio.NewReadWriter(bufio.NewReaderSize(rwc, 1048576), bufio.NewWriter(rwc)),
	}
}

// ListenAndServe starts listening and accepting requests to this server.
func (s *Server) ListenAndServe() error {
	addr := s.Addr
	if addr == "" {
		addr = ":11211"
	}
	l, e := net.Listen("tcp", addr)
	if e != nil {
		return e
	}
	return s.Serve(l)
}

func (s *Server) Serve(l net.Listener) error {
	defer l.Close()
	for {
		rw, e := l.Accept()
		if e != nil {
			return e
		}
		go s.newConn(rw).serve()
	}
}

func ListenAndServe(addr string) error {
	s := &Server{
		Addr: addr,
	}
	return s.ListenAndServe()
}

func parseStorageLine(line []byte) *StorageCmd {
	pieces := bytes.Fields(line) // Skip the actual "set "
	cmd := &StorageCmd{}
	// lol, no error handling here
	// TODO(jsfpdn): error handling.
	cmd.Key = string(pieces[0])
	cmd.Flags, _ = strconv.Atoi(string(pieces[1]))
	cmd.Exptime, _ = strconv.ParseInt(string(pieces[2]), 10, 64)
	cmd.Length, _ = strconv.Atoi(string(pieces[3]))
	cmd.Noreply = len(pieces) == 5 && bytes.Equal(pieces[4], noreply)
	return cmd
}

// NewServer initialize a new memcached Server.
func NewServer(listen string, handler RequestHandler) *Server {
	getter, _ := handler.(Getter)
	setter, _ := handler.(Setter)
	deleter, _ := handler.(Deleter)
	return &Server{listen, getter, setter, deleter, NewStats()}
}
