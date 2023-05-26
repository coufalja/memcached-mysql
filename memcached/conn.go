package memcached

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
)

type conn struct {
	server *Server
	conn   net.Conn
	rwc    *bufio.ReadWriter
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

type CommandType int

const (
	GetCmd CommandType = iota
	// Gat is get and touch
	GatCmd
	SetCmd
	StatsCmd
	DeleteCmd
	VersionCmd
	QuitCmd
	UnknownCmd
)

func parseCommand(command []byte) (CommandType, []byte) {
	if len(command) < 3 {
		return UnknownCmd, command
	}

	fields := bytes.Split(command, []byte(" "))
	commandName := string(fields[0])

	commandType := UnknownCmd
	switch commandName {
	case "get", "gets":
		commandType = GetCmd
	case "gat", "gats":
		commandType = GatCmd
	case "set":
		commandType = SetCmd
	case "stats":
		commandType = StatsCmd
	case "delete":
		commandType = DeleteCmd
	case "version":
		commandType = VersionCmd
	case "quit":
		commandType = QuitCmd
	}

	if commandType == UnknownCmd {
		return commandType, nil
	}

	return commandType, command[len(commandName):]
}

// handleRequest reads bytes from the connection, parses the command and
// calls appropriate handler.
func (c *conn) handleRequest() error {
	line, err := c.ReadLine()
	if err != nil || len(line) == 0 {
		return io.EOF
	}
	if len(line) < 4 {
		return Error
	}

	commandType, rest := parseCommand(line)

	switch commandType {
	case GetCmd, GatCmd:
		key := string(bytes.TrimSpace(line))
		return c.get(key)
	case SetCmd:
		// TODO: fix errors. Instead of Error, send SERVER_ERROR with custom message...
		return c.set(bytes.Trim(rest, " "))
	case StatsCmd:
		for key, value := range c.server.Stats.Snapshot() {
			fmt.Fprintf(c.rwc, StatusStat, key, value)
		}
		c.rwc.WriteString(StatusEnd)
		c.end()
	case DeleteCmd:
		return c.delete(string(line[7:]))
	case VersionCmd:
		c.rwc.WriteString(fmt.Sprintf(StatusVersion, VERSION))
		c.end()
	case QuitCmd:
		return io.EOF
	default:
		return Error
	}
	return nil
}

func (c *conn) get(key string) error {
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

	return nil
}

func (c *conn) set(line []byte) error {
	if c.server.Setter == nil {
		return Error
	}

	cmd := parseStorageLine(line)
	item := &Item{
		Key:   cmd.Key,
		Flags: cmd.Flags,
	}
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
		return nil
	}
	defer c.end()

	response := c.server.Setter.Set(item)
	if response != nil {
		response.WriteResponse(c.rwc)
	} else {
		c.rwc.WriteString(StatusStored)
	}

	return nil
}

func (c *conn) delete(key string) error {
	if c.server.Deleter == nil {
		return Error
	}

	if err := c.server.Deleter.Delete(key); err != nil {
		c.rwc.WriteString(StatusNotFound)
	} else {
		c.rwc.WriteString(StatusDeleted)
	}

	c.end()
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
