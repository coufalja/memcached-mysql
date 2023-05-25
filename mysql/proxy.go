package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coufalja/memcached-mysql/config"
	"github.com/coufalja/memcached-mysql/memcached"
)

const (
	mappingPrefix      = "@@"
	mappingSep         = "."
	defaultMapping     = "default"
	valueSeparator     = "|"
	columnSeparator    = ","
	tableNameSeparator = "."
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
		item, err := proxy.Get(ckey)
		if err != nil {
			return &memcached.ClientErrorResponse{Reason: err.Error()}
		}
		if item == nil {
			return nil
		}
		item.Key = key
		return &memcached.ItemResponse{Item: item}
	}
	return nil
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

func backtickSlice(elems []string) []string {
	newElems := make([]string, len(elems))

	for i, elem := range elems {
		newElems[i] = backtick(elem)
	}

	return newElems
}

func backtick(elem string) string {
	return fmt.Sprintf("`%s`", elem)
}

func formatSelectQuery(columns []string, table, keyColumn string) string {
	return fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s=?",
		strings.Join(backtickSlice(columns), columnSeparator),                                     // []string{"column1", "column2"} -> "`column1`,`column2`"
		strings.Join(backtickSlice(strings.Split(table, tableNameSeparator)), tableNameSeparator), // "database.table" ~> "`database`.`table`"
		backtick(keyColumn),
	)
}

func newTable(db *sql.DB, m config.Mapping) (*tableProxy, error) {
	columns := strings.Split(m.ValueColumn, valueSeparator)
	stmt, err := db.Prepare(formatSelectQuery(columns, m.Table, m.KeyColumn))
	if err != nil {
		return nil, err
	}
	return &tableProxy{query: stmt, columns: columns}, nil
}

type tableProxy struct {
	query   *sql.Stmt
	columns []string
}

func (c *tableProxy) Get(key string) (*memcached.Item, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	row := c.query.QueryRowContext(ctx, key)
	if row.Err() != nil {
		return nil, row.Err()
	}
	container := make([]sql.NullString, len(c.columns))
	pointers := make([]interface{}, len(c.columns))
	for i := range pointers {
		pointers[i] = &container[i]
	}
	if err := row.Scan(pointers...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	values := make([]string, len(c.columns))
	for i, c := range container {
		if c.Valid {
			values[i] = c.String
		}
	}
	return &memcached.Item{Value: []byte(strings.Join(values, valueSeparator))}, nil
}
