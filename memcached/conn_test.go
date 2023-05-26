package memcached

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type testHandler struct {
	kvs map[string]string
}

func (h *testHandler) Get(key string) MemcachedResponse {
	item := &Item{
		Key: key,
	}

	if value, ok := h.kvs[key]; ok {
		item.Value = []byte(value)
	}

	return &ItemResponse{
		Item: item,
	}
}

func (h *testHandler) Set(item *Item) MemcachedResponse {
	return nil
}

func (h *testHandler) Delete(key string) MemcachedResponse {
	if _, ok := h.kvs[key]; !ok {
		// To denote that item was not deleted since it is not present,
		// empty &ItemResponse should be returned.
		return &ItemResponse{}
	}

	return nil
}

func prepareConnection(fullCmd string, kvs map[string]string) *conn {
	handler := &testHandler{
		kvs: kvs,
	}

	return &conn{
		server: &Server{
			Getter:  handler,
			Deleter: handler,
			Setter:  handler,
			Stats:   NewStats(),
		},
		rwc: &bufio.ReadWriter{
			Reader: bufio.NewReader(strings.NewReader(fullCmd)),
		},
	}
}

func TestHandleRequest_Retrieval(t *testing.T) {
	tt := []struct {
		name    string
		conn    *conn
		wantErr error
		wantRes string
	}{
		{
			name:    "EOF",
			conn:    prepareConnection("", nil),
			wantErr: io.EOF,
		},
		{
			name:    "invalid command (command too short)",
			conn:    prepareConnection("ab", nil),
			wantErr: Error,
		},
		{
			name:    "invalid gits command",
			conn:    prepareConnection("gatis KEY", nil),
			wantErr: Error,
		},
		{
			name: "valid GET command (get)",
			conn: prepareConnection("get KEY", nil),
		},
		{
			name: "valid GET command (gat)",
			conn: prepareConnection("gat KEY", nil),
		},
		{
			name: "valid GET command (gets)",
			conn: prepareConnection("gets KEY", nil),
		},
		{
			name: "valid GET command (gats)",
			conn: prepareConnection("gats KEY", nil),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			b := bytes.Buffer{}
			tc.conn.rwc.Writer = bufio.NewWriter(&b)
			err := tc.conn.handleRequest()

			if tc.wantErr != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, b.Bytes())

				// TODO(jsfpdn): Fix race when accessing Stats.*.Count
				// require.Equal(t, tc.conn.server.Stats.CMDGet.Count, 1)
			}
		})
	}
}

func TestHandleRequest_Delete(t *testing.T) {
	tt := []struct {
		name    string
		conn    *conn
		wantErr error
		wantRes string
	}{
		{
			name:    "invalid DELETE command",
			conn:    prepareConnection("dehehe key", nil),
			wantErr: Error,
		},
		{
			name:    "valid DELETE command, key exists",
			conn:    prepareConnection("delete key", map[string]string{"key": "value"}),
			wantRes: StatusDeleted,
		},
		{
			name:    "valid DELETE command, key does not exist",
			conn:    prepareConnection("delete key", nil),
			wantRes: StatusNotFound,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			b := bytes.Buffer{}
			tc.conn.rwc.Writer = bufio.NewWriter(&b)
			err := tc.conn.handleRequest()

			if tc.wantErr != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantRes, b.String())
			}
		})
	}
}

func setCommand(key, value string) string {
	return fmt.Sprintf("set %s 1 1 %d\r\n%s\r\n", key, len(value), value)
}

func TestHandleRequest_Storage(t *testing.T) {
	tt := []struct {
		name    string
		conn    *conn
		wantErr error
		wantRes string
	}{
		{
			name:    "valid SET command",
			conn:    prepareConnection(setCommand("key", "value"), nil),
			wantRes: StatusStored,
		},
		{
			name:    "invalid payload size (too large)",
			conn:    prepareConnection("set key 0 0 1000\r\nvalue\r\n", nil),
			wantErr: ClientError,
		},
		{
			name:    "invalid payload size (too small)",
			conn:    prepareConnection("set key 0 0 1\r\nvalue\r\n", nil),
			wantErr: ClientError,
		},
		{
			name:    "missing crlf",
			conn:    prepareConnection("set key 0 0 5\r\nvalue", nil),
			wantErr: ClientError,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			b := bytes.Buffer{}
			tc.conn.rwc.Writer = bufio.NewWriter(&b)
			err := tc.conn.handleRequest()

			if tc.wantErr != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, b.Bytes())
				require.Equal(t, tc.wantRes, b.String())
			}
		})
	}
}

func TestHandleRequest_Quit(t *testing.T) {
	conn := prepareConnection("quit", nil)
	b := bytes.Buffer{}
	conn.rwc.Writer = bufio.NewWriter(&b)

	require.Error(t, io.EOF, conn.handleRequest())
	require.Equal(t, b.String(), "")
}

func TestHandleRequest_Version(t *testing.T) {
	conn := prepareConnection("version", nil)
	b := bytes.Buffer{}
	conn.rwc.Writer = bufio.NewWriter(&b)

	require.NoError(t, conn.handleRequest())
	require.Equal(t, b.String(), fmt.Sprintf(StatusVersion, VERSION))
}

func TestHandleRequest_Stats(t *testing.T) {
	conn := prepareConnection("stats", nil)
	b := bytes.Buffer{}
	conn.rwc.Writer = bufio.NewWriter(&b)

	require.NoError(t, conn.handleRequest())
	require.NotEmpty(t, b.String())
}

func TestParseCommand(t *testing.T) {
	tt := []struct {
		name     string
		input    []byte
		wantCmd  CommandType
		wantRest []byte
	}{
		{
			name:     "command too short - invalid command",
			input:    []byte("ab"),
			wantCmd:  UnknownCmd,
			wantRest: []byte("ab"),
		},
		{
			name:     "only get",
			input:    []byte("get"),
			wantCmd:  GetCmd,
			wantRest: []byte(""),
		},
		{
			name:     "get with trailing whitespace",
			input:    []byte("get  "),
			wantCmd:  GetCmd,
			wantRest: []byte("  "),
		},
		{
			name:     "get with key supplied",
			input:    []byte("get key"),
			wantCmd:  GetCmd,
			wantRest: []byte(" key"),
		},
		{
			name:     "gats with key supplied",
			input:    []byte("gats key"),
			wantCmd:  GatCmd,
			wantRest: []byte(" key"),
		},
		{
			name:     "version",
			input:    []byte("version"),
			wantCmd:  VersionCmd,
			wantRest: []byte(""),
		},
		{
			name:     "quit",
			input:    []byte("quit"),
			wantCmd:  QuitCmd,
			wantRest: []byte(""),
		},
		{
			name:     "set with flags",
			input:    []byte("set key 0 0 1\r\nvalue\r\n"),
			wantCmd:  SetCmd,
			wantRest: []byte(" key 0 0 1\r\nvalue\r\n"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			gotCmd, gotRest := parseCommand(tc.input)
			require.Equal(t, tc.wantCmd, gotCmd)
			require.Equal(t, tc.wantRest, gotRest)
		})
	}
}

func TestParseSetArgs(t *testing.T) {
	tt := []struct {
		name     string
		input    []byte
		wantArgs setArgs
		wantErr  error
	}{
		{
			name:    "nil input",
			input:   nil,
			wantErr: ClientError,
		},
		{
			name:    "incorrect flags",
			input:   []byte("key bad 1 10"),
			wantErr: ClientError,
		},
		{
			name:    "incorrect exptime",
			input:   []byte("key 1 bad 10"),
			wantErr: ClientError,
		},
		{
			name:    "incorrect data block size",
			input:   []byte("key 1 1 bad"),
			wantErr: ClientError,
		},
		{
			name:  "correct arguments",
			input: []byte("key 1 1 10"),
			wantArgs: setArgs{
				key:     []byte("key"),
				flags:   1,
				exptime: 1,
				bytes:   10,
			},
		},
		{
			name:  "correct arguments with noreply",
			input: []byte("key 1 1 10 noreply"),
			wantArgs: setArgs{
				key:     []byte("key"),
				flags:   1,
				exptime: 1,
				bytes:   10,
				noReply: true,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			gotArgs, gotErr := parseSetArgs(tc.input)
			require.Equal(t, tc.wantArgs, gotArgs)
			if tc.wantErr != nil {
				require.Error(t, gotErr)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}
