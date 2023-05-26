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
		// TODO(jsfpdn): This test case should fail, since "gits" is not a valid command.
		//		{
		//			name: "valid GET command (gits)",
		//			conn: &conn{
		//				server: testServer(),
		//				rwc: &bufio.ReadWriter{
		//					Reader: bufio.NewReader((strings.NewReader("gats KEY"))),
		//				},
		//			},
		//			wantErr: Error,
		//		},
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
		// TODO(jsfpdn): Return error when encountering such command.
		// 		{
		// 			name:    "invalid DELETE command",
		//			conn:    prepareConnection(value, "dehehe key", nil),
		//			wantErr: Error,
		//		},
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
	return fmt.Sprintf("set %s 0 0 %d\n%s\r\n", key, len(value), value)
}

func TestHandleRequest_Storage(t *testing.T) {
	tt := []struct {
		name    string
		conn    *conn
		wantErr error
		wantRes string
	}{
		// TODO(jsfpdn): Return error when encountering such command.
		// 		{
		// 			name:    "invalid SET command",
		//			conn:    prepareConnection(value, "sed key value", nil),
		//			wantErr: Error,
		//		},

		// TODO(jsfpdn): Fix panic when inccorect number of fields is supplied.
		// {
		//			name:    "valid SET command, incorrect number of fields",
		//			conn:    prepareConnection("set key 0 1000\nvalue", nil),
		//			wantErr: Error,
		//		},
		{
			name:    "valid SET command",
			conn:    prepareConnection(setCommand("key", "value"), nil),
			wantRes: StatusStored,
		},
		{
			name:    "valid SET command, invalid payload size",
			conn:    prepareConnection("set key 0 0 1000\nvalue\r\n", nil),
			wantErr: Error,
		},
		{
			name:    "valid SET command, missing suffix",
			conn:    prepareConnection("set key 0 0 1000\nvalue", nil),
			wantErr: Error,
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
			input:    []byte("set key 0 0 1\nvalue\r\n"),
			wantCmd:  SetCmd,
			wantRest: []byte(" key 0 0 1\nvalue\r\n"),
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
