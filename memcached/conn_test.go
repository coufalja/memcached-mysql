package memcached

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type testHandler struct {
	value string
}

func (h *testHandler) Get(key string) MemcachedResponse {
	return &ItemResponse{
		Item: &Item{
			Key:   key,
			Value: []byte(h.value),
		},
	}
}

func prepareConnection(value, fullCmd string) *conn {
	return &conn{
		server: &Server{
			Getter: &testHandler{value: value},
			Stats:  NewStats(),
		},
		rwc: &bufio.ReadWriter{
			Reader: bufio.NewReader(strings.NewReader(fullCmd)),
		},
	}
}

func TestHandleRequest_Retrieval(t *testing.T) {
	const value = "return_value"

	tt := []struct {
		name    string
		conn    *conn
		wantErr error
		wantRes string
	}{
		{
			name:    "EOF",
			conn:    prepareConnection(value, ""),
			wantErr: io.EOF,
		},
		{
			name:    "invalid command (command too short)",
			conn:    prepareConnection(value, "ab"),
			wantErr: Error,
		},
		{
			name: "valid GET command (get)",
			conn: prepareConnection(value, "get KEY"),
		},
		{
			name: "valid GET command (gat)",
			conn: prepareConnection(value, "gat KEY"),
		},
		{
			name: "valid GET command (gets)",
			conn: prepareConnection(value, "gets KEY"),
		},
		{
			name: "valid GET command (gats)",
			conn: prepareConnection(value, "gats KEY"),
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

func TestHandleRequest_Storage(t *testing.T) {}
