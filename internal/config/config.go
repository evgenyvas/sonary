// Package config
package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"log"
	"os"
)

type Config struct {
	Host     string `env:"HOST"`
	AppEnv   string `env:"APP_ENV"`
	RootPath string `env:"ROOT_PATH"`
}

var instance *Config

func init() {
	// read configuration
	if err := godotenv.Load(); err != nil { // .env
		log.Println("No .env file found")
	}
	godotenv.Load(".env.local")
	appEnv := os.Getenv("APP_ENV")
	if err := godotenv.Overload(".env." + appEnv); err != nil {
		log.Println("No .env." + appEnv + " file found")
	}
	godotenv.Load(".env.local")

	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		log.Fatalf("Failed to parse environment variables: %v", err)
	}
	instance = &cfg
}

func GetConfig() *Config {
	return instance
}
