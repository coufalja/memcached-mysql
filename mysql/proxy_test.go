package mysql

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/coufalja/memcached-mysql/config"
	"github.com/coufalja/memcached-mysql/memcached"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	type args struct {
		mapping []config.Mapping
	}
	tests := []struct {
		name string
		args args
		mock func(sqlmock.Sqlmock)
	}{
		{
			name: "empty mapping",
			args: args{
				mapping: nil,
			},
		},
		{
			name: "single mapping",
			args: args{
				mapping: []config.Mapping{
					{
						Name:        "test",
						KeyColumn:   "key",
						ValueColumn: "value",
						Table:       "test",
					},
				},
			},
			mock: func(s sqlmock.Sqlmock) {
				s.ExpectPrepare("SELECT `value` FROM `test` WHERE `key`=?")
			},
		},
		{
			name: "multi mapping",
			args: args{
				mapping: []config.Mapping{
					{
						Name:        "test",
						KeyColumn:   "key",
						ValueColumn: "value",
						Table:       "test",
					},
					{
						Name:        "test2",
						KeyColumn:   "key",
						ValueColumn: "value",
						Table:       "test2",
					},
				},
			},
			mock: func(s sqlmock.Sqlmock) {
				s.ExpectPrepare("SELECT `value` FROM `test` WHERE `key`=?")
				s.ExpectPrepare("SELECT `value` FROM `test2` WHERE `key`=?")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if tt.mock != nil {
				tt.mock(mock)
			}
			require.NoError(t, err)
			require.NotPanics(t, func() {
				New(db, tt.args.mapping)
			})
		})
	}
}

func TestProxy_Get(t *testing.T) {
	type fields struct {
		mappings []config.Mapping
	}
	type args struct {
		key string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		mock   func(sqlmock.Sqlmock)
		want   memcached.MemcachedResponse
	}{
		{
			name: "raw key uses default mapping",
			fields: fields{mappings: []config.Mapping{
				{
					Name:        "default",
					KeyColumn:   "key",
					ValueColumn: "value",
					Table:       "test",
				},
				{
					Name:        "foo",
					KeyColumn:   "key",
					ValueColumn: "value",
					Table:       "fooTable",
				},
			}},
			mock: func(s sqlmock.Sqlmock) {
				s.ExpectPrepare("SELECT `value` FROM `test` WHERE `key`=?")
				s.ExpectPrepare("SELECT `value` FROM `fooTable` WHERE `key`=?")
				s.ExpectQuery("SELECT `value` FROM `test` WHERE `key`=.+").WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("bar"))
			},
			args: args{key: "key"},
			want: &memcached.ItemResponse{Item: &memcached.Item{
				Key:   "key",
				Value: []byte("bar"),
			}},
		},
		{
			name: "unknown key prefix",
			fields: fields{mappings: []config.Mapping{
				{
					Name:        "default",
					KeyColumn:   "key",
					ValueColumn: "value",
					Table:       "test",
				},
				{
					Name:        "foo",
					KeyColumn:   "key",
					ValueColumn: "value",
					Table:       "fooTable",
				},
			}},
			mock: func(s sqlmock.Sqlmock) {
				s.ExpectPrepare("SELECT `value` FROM `test` WHERE `key`=?")
				s.ExpectPrepare("SELECT `value` FROM `fooTable` WHERE `key`=?")
			},
			args: args{key: "@@unknown.key"},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, s, err := sqlmock.New()
			require.NoError(t, err)
			if tt.mock != nil {
				tt.mock(s)
			}
			c := New(db, tt.fields.mappings)
			got := c.Get(tt.args.key)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_mappingKey(t *testing.T) {
	type args struct {
		key string
	}
	tests := []struct {
		name         string
		args         args
		wantMapping  string
		wantPlainKey string
		wantErr      bool
	}{
		{
			name:         "plain key",
			args:         args{key: "key"},
			wantMapping:  defaultMapping,
			wantPlainKey: "key",
		},
		{
			name:         "scoped key",
			args:         args{key: "@@aa.key"},
			wantMapping:  "aa",
			wantPlainKey: "key",
		},
		{
			name:    "invalid key",
			args:    args{key: "@@aaaaa"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			got, got1, err := mappingKey(tt.args.key)
			if tt.wantErr {
				r.Error(err)
				return
			} else {
				r.NoError(err)
			}
			r.Equal(tt.wantMapping, got)
			r.Equal(tt.wantPlainKey, got1)
		})
	}
}

func Test_tableProxy_Get(t *testing.T) {
	type fields struct {
		mapping config.Mapping
	}
	type args struct {
		key string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		mock   func(sqlmock.Sqlmock)
		want   memcached.MemcachedResponse
	}{
		{
			name: "key found",
			fields: fields{
				mapping: config.Mapping{
					Name:        "default",
					KeyColumn:   "key",
					ValueColumn: "value",
					Table:       "test",
				},
			},
			args: args{key: "foo"},
			mock: func(s sqlmock.Sqlmock) {
				s.ExpectPrepare("SELECT `value` FROM `test` WHERE `key`=?")
				s.ExpectQuery("SELECT `value` FROM `test` WHERE `key`=.*").WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("bar"))
			},
			want: &memcached.ItemResponse{Item: &memcached.Item{
				Key:   "foo",
				Value: []byte("bar"),
			}},
		},
		{
			name: "key found multiple values",
			fields: fields{
				mapping: config.Mapping{
					Name:        "default",
					KeyColumn:   "key",
					ValueColumn: "value|value2",
					Table:       "test",
				},
			},
			args: args{key: "foo"},
			mock: func(s sqlmock.Sqlmock) {
				s.ExpectPrepare("SELECT `value`,`value2` FROM `test` WHERE `key`=?")
				s.ExpectQuery("SELECT `value`,`value2` FROM `test` WHERE `key`=.*").WillReturnRows(sqlmock.NewRows([]string{"value", "valu2"}).AddRow("bar", "bar2"))
			},
			want: &memcached.ItemResponse{Item: &memcached.Item{
				Key:   "foo",
				Value: []byte("bar|bar2"),
			}},
		},
		{
			name: "key not found",
			fields: fields{
				mapping: config.Mapping{
					Name:        "default",
					KeyColumn:   "key",
					ValueColumn: "value",
					Table:       "test",
				},
			},
			args: args{key: "foo"},
			mock: func(s sqlmock.Sqlmock) {
				s.ExpectPrepare("SELECT `value` FROM `test` WHERE `key`=?")
				s.ExpectQuery("SELECT `value` FROM `test` WHERE `key`=.*").WillReturnRows(sqlmock.NewRows([]string{"value"}))
			},
			want: nil,
		},
		{
			name: "query failed",
			fields: fields{
				mapping: config.Mapping{
					Name:        "default",
					KeyColumn:   "key",
					ValueColumn: "value",
					Table:       "test",
				},
			},
			args: args{key: "foo"},
			mock: func(s sqlmock.Sqlmock) {
				s.ExpectPrepare("SELECT `value` FROM `test` WHERE `key`=?")
				s.ExpectQuery("SELECT `value` FROM `test` WHERE `key`=.*").WillReturnError(errors.New("unknown error"))
			},
			want: &memcached.ClientErrorResponse{Reason: "unknown error"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			if tt.mock != nil {
				tt.mock(mock)
			}
			c, err := newTable(db, tt.fields.mapping)
			require.NoError(t, err)
			got := c.Get(tt.args.key)
			require.Equal(t, tt.want, got)
		})
	}
}
