package config

import (
	"fmt"
	"os"
	"time"
)

type Server struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// mysqlConnectionTmpl is a MySQL connection string template in the form
// <user>:<password>@tcp(<host>:<port>).
const mysqlConnectionTmpl = "%s:%s@tcp(%s:%d)"

type MySQL struct {
	Connection      string
	Password        string        `json:"password"`
	User            string        `json:"user"`
	Host            string        `json:"host"`
	Port            int           `json:"port"`
	Database        string        `json:"database"`
	ConnMaxLifetime time.Duration `json:"connMaxLifetime"`
	MaxOpenConns    int           `json:"maxOpenConns"`
	MaxIdleConns    int           `json:"maxIdleConns"`
}

type Config struct {
	Server  Server    `json:"server"`
	MySQL   MySQL     `json:"mysql"`
	Mapping []Mapping `json:"mapping"`
}

type Mapping struct {
	Name        string `json:"name"`
	KeyColumn   string `json:"keyColumn"`
	ValueColumn string `json:"valueColumn"`
	Table       string `json:"table"`
}

func (c *Mapping) EnsureDefault() {
	if c.Name == "" {
		c.Name = "default"
	}
	if c.KeyColumn == "" {
		c.KeyColumn = "key"
	}
	if c.ValueColumn == "" {
		c.ValueColumn = "value"
	}
}

func (c *Config) EnsureDefault() {
	if c.Server.Port == 0 {
		c.Server.Port = 11211
	}

	c.MySQL.User = os.ExpandEnv(c.MySQL.User)

	c.MySQL.Connection = fmt.Sprintf(mysqlConnectionTmpl, c.MySQL.User, c.MySQL.Password, c.MySQL.Host, c.MySQL.Port)
	if c.MySQL.Database != "" {
		c.MySQL.Connection = fmt.Sprintf("%s/%s", c.MySQL.Connection, c.MySQL.Database)
	}

	if c.MySQL.ConnMaxLifetime == 0 {
		c.MySQL.ConnMaxLifetime = 3 * time.Minute
	}

	if c.MySQL.MaxIdleConns == 0 {
		c.MySQL.MaxIdleConns = -1
	}

	if c.MySQL.MaxOpenConns == 0 {
		c.MySQL.MaxOpenConns = -1
	}

	for i := range c.Mapping {
		c.Mapping[i].EnsureDefault()
	}
}
