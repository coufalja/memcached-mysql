package main

import (
	"database/sql"
	"fmt"

	"github.com/coufalja/memcached-mysql/config"
	"github.com/coufalja/memcached-mysql/mysql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/mattrobenolt/go-memcached"
	"github.com/spf13/viper"
)

func main() {
	c := config.Config{}
	if err := viper.Unmarshal(&c); err != nil {
		panic(err)
	}
	c.EnsureDefault()
	db, err := sql.Open("mysql", c.MySQL.Connection)
	if err != nil {
		panic(err)
	}
	db.SetConnMaxLifetime(c.MySQL.ConnMaxLifetime)
	db.SetMaxOpenConns(c.MySQL.MaxOpenConns)
	db.SetMaxIdleConns(c.MySQL.MaxIdleConns)
	proxy := memcached.NewServer(fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port), mysql.New(db, c.Mapping))
	if err := proxy.ListenAndServe(); err != nil {
		panic(err)
	}
}

func init() {
	viper.AutomaticEnv()
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("config")
}
