package config

import (
	"flag"
	"os"
)

type Config struct {
	ServerAddress        string
	DatabaseDSN          string
	AccrualSystemAddress string
}

func New() *Config {
	return &Config{
		ServerAddress:        "localhost:8080",
		DatabaseDSN:          "postgresql://postgres:postgres@localhost:5432/gophermart?sslmode=disable",
		AccrualSystemAddress: "localhost:8081",
	}
}

func (c *Config) ParseFlags() {
	flag.StringVar(&c.ServerAddress, "a", c.ServerAddress, "server address")
	flag.StringVar(&c.DatabaseDSN, "d", c.DatabaseDSN, "database URI")
	flag.StringVar(&c.AccrualSystemAddress, "r", c.AccrualSystemAddress, "accrual system address")
	flag.Parse()
}

func (c *Config) ParseEnv() {
	if addr := os.Getenv("RUN_ADDRESS"); addr != "" {
		c.ServerAddress = addr
	}
	if dsn := os.Getenv("DATABASE_URI"); dsn != "" {
		c.DatabaseDSN = dsn
	}
	if acc := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); acc != "" {
		c.AccrualSystemAddress = acc
	}
}
