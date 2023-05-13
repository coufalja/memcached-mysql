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

type MultiProxy struct {
	p map[string]*SingleProxy
}

func (c *MultiProxy) Get(key string) memcached.MemcachedResponse {
	mapping, ckey, err := mappingKey(key)
	if err != nil {
		return &memcached.ClientErrorResponse{Reason: err.Error()}
	}
	if proxy, ok := c.p[mapping]; ok {
		return proxy.Get(ckey)
	}
	return &memcached.ClientErrorResponse{Reason: fmt.Sprintf("no mapping present for a key: '%s'", key)}
}

func mappingKey(key string) (string, string, error) {
	if strings.HasPrefix(key, "@@") {
		sep := strings.Split(key, ".")
		if len(sep) < 2 {
			return "", "", errors.New("bad key format")
		}
		return strings.TrimLeft(sep[0], "@"), sep[1], nil
	}
	return "default", key, nil
}

func New(db *sql.DB, mapping []config.Mapping) *MultiProxy {
	proxy := &MultiProxy{
		p: make(map[string]*SingleProxy),
	}
	for _, m := range mapping {
		proxy.p[m.Name] = &SingleProxy{
			query: fmt.Sprintf("SELECT %s FROM %s WHERE %s=?", m.ValueColumn, m.Table, m.KeyColumn),
			db:    db,
		}
	}
	return proxy
}

type SingleProxy struct {
	query string
	db    *sql.DB
}

func (c *SingleProxy) Get(key string) memcached.MemcachedResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	row := c.db.QueryRowContext(ctx, c.query, key)
	if row.Err() != nil {
		return &memcached.ClientErrorResponse{Reason: row.Err().Error()}
	}
	res := ""
	err := row.Scan(&res)
	if err != nil {
		return &memcached.ClientErrorResponse{Reason: err.Error()}
	}
	return &memcached.ItemResponse{Item: &memcached.Item{Key: key, Value: []byte(res)}}
}
