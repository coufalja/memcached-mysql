package memcached

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
)

type conn struct {
	server *Server
	conn   net.Conn
	rwc    *bufio.ReadWriter
}

func (c *conn) serve() {
	defer func() {
		c.server.Stats.CurrConnections.Add(-1)
		c.Close()
	}()
	c.server.Stats.TotalConnections.Add(1)
	c.server.Stats.CurrConnections.Add(1)
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
	// Gat is get and touch, refreshing the expiration time.
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
		return c.set(bytes.TrimLeft(rest, " "))
	case StatsCmd:
		c.stats()
	case DeleteCmd:
		key := string(bytes.Trim(rest, " "))
		return c.delete(key)
	case VersionCmd:
		c.version()
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

	c.server.Stats.CMDGet.Add(1)
	response := c.server.Getter.Get(key)
	if response != nil {
		c.server.Stats.GetHits.Add(1)
		response.WriteResponse(c.rwc)
	} else {
		c.server.Stats.GetMisses.Add(1)
	}
	c.rwc.WriteString(StatusEnd)
	c.end()

	return nil
}

type setArgs struct {
	key     []byte
	flags   int
	exptime int
	bytes   int
	noReply bool
}

func parseSetArgs(arguments []byte) (setArgs, error) {
	argFields := bytes.Split(arguments, []byte(" "))
	if len(argFields) < 4 {
		return setArgs{}, fmt.Errorf(ClientError.Error(), "set has incorrect number of arguments")
	}

	args := setArgs{
		key: argFields[0],
	}

	if len(argFields) == 5 {
		if !bytes.Equal(argFields[4], noreply) {
			return setArgs{}, fmt.Errorf(ClientError.Error(), "last set argument must be empty or noreply")
		}
		args.noReply = true
	}

	var err error
	if args.flags, err = strconv.Atoi(string(argFields[1])); err != nil {
		return setArgs{}, fmt.Errorf(ClientError.Error(), fmt.Sprintf("could not parse flags: %s", err))
	}

	if args.exptime, err = strconv.Atoi(string(argFields[2])); err != nil {
		return setArgs{}, fmt.Errorf(ClientError.Error(), fmt.Sprintf("could not parse exptime: %s", err))
	}

	if args.bytes, err = strconv.Atoi(string(argFields[3])); err != nil {
		return setArgs{}, fmt.Errorf(ClientError.Error(), fmt.Sprintf("could not parse data block size: %s", err))
	}

	return args, nil
}

func (c *conn) set(line []byte) error {
	if c.server.Setter == nil {
		return ServerError
	}

	args, err := parseSetArgs(line)
	if err != nil {
		return err
	}

	item := &Item{
		Key:   string(args.key),
		Flags: args.flags,
	}
	item.SetExpires(int64(args.exptime))

	value := make([]byte, args.bytes+len(crlf))
	n, err := c.Read(value)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf(ClientError.Error(), fmt.Sprintf("payload is smaller than provided payload size: %s", err))
	}

	if err != nil {
		return fmt.Errorf(ServerError.Error(), fmt.Sprintf("could not read data block: %s", err))
	}

	if n != args.bytes+len(crlf) {
		return fmt.Errorf(ServerError.Error(), fmt.Sprintf("data block is of size %d instead of %d", n, args.bytes))
	}

	if !bytes.HasSuffix(value, crlf) {
		return fmt.Errorf(ServerError.Error(), "data block does not end with \\r\\n")
	}

	// Copy the value into the *Item
	item.Value = make([]byte, len(value)-2)
	copy(item.Value, value)

	c.server.Stats.CMDSet.Add(1)
	if args.noReply {
		go c.server.Setter.Set(item)
		return nil
	}

	response := c.server.Setter.Set(item)
	if response != nil {
		response.WriteResponse(c.rwc)
	} else {
		c.rwc.WriteString(StatusStored)
	}

	c.end()
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

func (c *conn) stats() {
	for key, value := range c.server.Stats.Snapshot() {
		fmt.Fprintf(c.rwc, StatusStat, key, value)
	}
	c.rwc.WriteString(StatusEnd)
	c.end()
}

func (c *conn) version() {
	c.rwc.WriteString(fmt.Sprintf(StatusVersion, VERSION))
	c.end()
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
