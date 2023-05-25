// Package memcached provides an interface for building your
// own memcached ascii protocol servers.
package memcached

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

const VERSION = "0.0.0"

var (
	crlf    = []byte("\r\n")
	noreply = []byte("noreply")
)

type conn struct {
	server *Server
	conn   net.Conn
	rwc    *bufio.ReadWriter
}

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

func (s *Server) newConn(rwc net.Conn) (c *conn) {
	c = new(conn)
	c.server = s
	c.conn = rwc
	c.rwc = bufio.NewReadWriter(bufio.NewReaderSize(rwc, 1048576), bufio.NewWriter(rwc))
	return c
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
		c := s.newConn(rw)
		go c.serve()
	}
}

func (c *conn) serve() {
	defer func() {
		c.server.Stats.CurrConnections.Decrement(1)
		c.Close()
	}()
	c.server.Stats.TotalConnections.Increment(1)
	c.server.Stats.CurrConnections.Increment(1)
	for {
		err := c.handleRequest()
		if err != nil {
			if err == io.EOF {
				return
			}
			c.rwc.WriteString(err.Error())
			c.end()
		}
	}
}

func (c *conn) end() {
	c.rwc.Flush()
}

func (c *conn) handleRequest() error {
	line, err := c.ReadLine()
	if err != nil || len(line) == 0 {
		return io.EOF
	}
	if len(line) < 4 {
		return Error
	}
	switch line[0] {
	case 'g':
		f := strings.Fields(string(line))
		if len(f) != 2 {
			return Error
		}
		key := f[1]

		if c.server.Getter == nil {
			return Error
		}
		c.server.Stats.CMDGet.Increment(1)
		response := c.server.Getter.Get(key)
		if response != nil {
			c.server.Stats.GetHits.Increment(1)
			response.WriteResponse(c.rwc)
		} else {
			c.server.Stats.GetMisses.Increment(1)
		}
		c.rwc.WriteString(StatusEnd)
		c.end()
	case 's':
		switch line[1] {
		case 'e':
			if len(line) < 11 {
				return Error
			}
			if c.server.Setter == nil {
				return Error
			}
			item := &Item{}
			cmd := parseStorageLine(line)
			item.Key = cmd.Key
			item.Flags = cmd.Flags
			item.SetExpires(cmd.Exptime)

			value := make([]byte, cmd.Length+2)
			n, err := c.Read(value)
			if err != nil {
				return Error
			}

			// Didn't provide the correct number of bytes
			if n != cmd.Length+2 {
				response := &ClientErrorResponse{"bad chunk data"}
				response.WriteResponse(c.rwc)
				c.ReadLine() // Read out the rest of the line
				return Error
			}

			// Doesn't end with \r\n
			if !bytes.HasSuffix(value, crlf) {
				response := &ClientErrorResponse{"bad chunk data"}
				response.WriteResponse(c.rwc)
				c.ReadLine() // Read out the rest of the line
				return Error
			}

			// Copy the value into the *Item
			item.Value = make([]byte, len(value)-2)
			copy(item.Value, value)

			c.server.Stats.CMDSet.Increment(1)
			if cmd.Noreply {
				go c.server.Setter.Set(item)
			} else {
				response := c.server.Setter.Set(item)
				if response != nil {
					response.WriteResponse(c.rwc)
					c.end()
				} else {
					c.rwc.WriteString(StatusStored)
					c.end()
				}
			}
		case 't':
			if len(line) != 5 {
				return Error
			}
			for key, value := range c.server.Stats.Snapshot() {
				fmt.Fprintf(c.rwc, StatusStat, key, value)
			}
			c.rwc.WriteString(StatusEnd)
			c.end()
		default:
			return Error
		}
	case 'd':
		if len(line) < 8 {
			return Error
		}
		key := string(line[7:]) // delete
		if c.server.Deleter == nil {
			return Error
		}
		err := c.server.Deleter.Delete(key)
		if err != nil {
			c.rwc.WriteString(StatusNotFound)
			c.end()
		} else {
			c.rwc.WriteString(StatusDeleted)
			c.end()
		}
	case 'v':
		if len(line) != 7 {
			return Error
		}
		c.rwc.WriteString(fmt.Sprintf(StatusVersion, VERSION))
		c.end()
	case 'q':
		if len(line) == 4 {
			return io.EOF
		}
		return Error
	default:
		return Error
	}
	return nil
}

func (c *conn) Close() {
	c.conn.Close()
}

func (c *conn) ReadLine() (line []byte, err error) {
	line, _, err = c.rwc.ReadLine()
	return
}

func (c *conn) Read(p []byte) (n int, err error) {
	return io.ReadFull(c.rwc, p)
}

func parseStorageLine(line []byte) *StorageCmd {
	pieces := bytes.Fields(line[4:]) // Skip the actual "set "
	cmd := &StorageCmd{}
	// lol, no error handling here
	cmd.Key = string(pieces[0])
	cmd.Flags, _ = strconv.Atoi(string(pieces[1]))
	cmd.Exptime, _ = strconv.ParseInt(string(pieces[2]), 10, 64)
	cmd.Length, _ = strconv.Atoi(string(pieces[3]))
	cmd.Noreply = len(pieces) == 5 && bytes.Equal(pieces[4], noreply)
	return cmd
}

// Initialize a new memcached Server.
func NewServer(listen string, handler RequestHandler) *Server {
	return &Server{listen, handler.(Getter), handler.(Setter), handler.(Deleter), NewStats()}
}
