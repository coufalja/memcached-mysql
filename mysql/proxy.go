package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coufalja/memcached-mysql/config"
	"github.com/mattrobenolt/go-memcached"
)

const (
	mappingPrefix  = "@@"
	mappingSep     = "."
	defaultMapping = "default"
	valueSeparator = "|"
)

type Proxy struct {
	tables map[string]*tableProxy
}

func (c *Proxy) Get(key string) memcached.MemcachedResponse {
	mapping, ckey, err := mappingKey(key)
	if err != nil {
		return &memcached.ClientErrorResponse{Reason: err.Error()}
	}
	if proxy, ok := c.tables[mapping]; ok {
		return proxy.Get(ckey)
	}
	return &memcached.ClientErrorResponse{Reason: fmt.Sprintf("no mapping present for a key: '%s'", key)}
}

func mappingKey(key string) (string, string, error) {
	if strings.HasPrefix(key, mappingPrefix) {
		sep := strings.Split(key, mappingSep)
		if len(sep) < 2 {
			return "", "", errors.New("bad key format")
		}
		return strings.TrimLeft(sep[0], "@"), sep[1], nil
	}
	return defaultMapping, key, nil
}

func New(db *sql.DB, mapping []config.Mapping) *Proxy {
	proxy := &Proxy{
		tables: make(map[string]*tableProxy),
	}
	for _, m := range mapping {
		tp, err := newTable(db, m)
		if err != nil {
			panic(err)
		}
		proxy.tables[m.Name] = tp
	}
	return proxy
}

func newTable(db *sql.DB, m config.Mapping) (*tableProxy, error) {
	columns := strings.Split(m.ValueColumn, valueSeparator)
	stmt, err := db.Prepare(fmt.Sprintf("SELECT (%s) FROM %s WHERE %s=?", strings.Join(columns, ","), m.Table, m.KeyColumn))
	if err != nil {
		return nil, err
	}
	return &tableProxy{query: stmt, columns: columns}, nil
}

type tableProxy struct {
	query   *sql.Stmt
	columns []string
}

func (c *tableProxy) Get(key string) memcached.MemcachedResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	row := c.query.QueryRowContext(ctx, key)
	if row.Err() != nil {
		return &memcached.ClientErrorResponse{Reason: row.Err().Error()}
	}
	container := make([]string, len(c.columns))
	pointers := make([]interface{}, len(c.columns))
	for i := range pointers {
		pointers[i] = &container[i]
	}
	if err := row.Scan(pointers...); err != nil {
		return &memcached.ClientErrorResponse{Reason: err.Error()}
	}
	return &memcached.ItemResponse{Item: &memcached.Item{Key: key, Value: []byte(strings.Join(container, valueSeparator))}}
}
