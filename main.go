package main

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/coufalja/memcached-mysql/config"
	"github.com/coufalja/memcached-mysql/memcached"
	"github.com/coufalja/memcached-mysql/mysql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	db, err := sql.Open("mysql", conf.MySQL.Connection)
	if err != nil {
		logger.Panic("failed to open mysql connection", zap.Error(err))
	}
	db.SetConnMaxLifetime(conf.MySQL.ConnMaxLifetime)
	db.SetMaxOpenConns(conf.MySQL.MaxOpenConns)
	db.SetMaxIdleConns(conf.MySQL.MaxIdleConns)
	if err := db.Ping(); err != nil {
		logger.Panic("could not connect to the mysql server", zap.Error(err))
	}

	proxy := memcached.NewServer(fmt.Sprintf("%s:%d", conf.Server.Host, conf.Server.Port), mysql.New(db, conf.Mapping))
	if err := proxy.ListenAndServe(); err != nil {
		logger.Panic("failed to start server", zap.Error(err))
	}
	logger.Info("memcached proxy started")
}

var (
	logger *zap.Logger
	conf   config.Config
)

func init() {
	pflag.String("config", "config.yaml", "Path to a config file.")

	pflag.Parse()

	viper.BindPFlags(pflag.CommandLine)
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetConfigFile(viper.GetString("config"))

	l, err := zap.NewProductionConfig().Build()
	if err != nil {
		panic(err)
	}
	logger = l

	if err := viper.ReadInConfig(); err != nil {
		logger.Error("failed to read in config", zap.Error(err))
	}
	c := config.Config{}
	if err := viper.Unmarshal(&c); err != nil {
		logger.Error("failed to unmarshal config", zap.Error(err))
	}
	c.EnsureDefault()
	conf = c
}
