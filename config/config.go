package config

import "time"

type Server struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type MySQL struct {
	Connection      string        `json:"connection"`
	ConnMaxLifetime time.Duration `json:"maxLifetime"`
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

	if c.MySQL.Connection == "" {
		c.MySQL.Connection = "mysql:mysql@/mysql"
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

	for _, mapping := range c.Mapping {
		mapping.EnsureDefault()
	}
}
