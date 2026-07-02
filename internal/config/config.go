// Package config
package config

import (
	"encoding/json"
	"log"
	"os"
	"reflect"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Host        string   `env:"HOST"`
	AppEnv      string   `env:"APP_ENV"`
	RootPaths   []string `env:"ROOT_PATHS"`
	DatabaseDsn string   `env:"DATABASE_DSN"`
	WorkerCount int      `env:"WORKER_COUNT"`
}

var instance *Config

func init() {
	// read configuration
	if err := godotenv.Load(); err != nil { // .env
		log.Println("No .env file found")
	}
	godotenv.Overload(".env.local")
	appEnv := os.Getenv("APP_ENV")
	if err := godotenv.Overload(".env." + appEnv); err != nil {
		log.Println("No .env." + appEnv + " file found")
	}
	godotenv.Overload(".env.local")

	cfg := Config{}
	options := env.Options{
		FuncMap: map[reflect.Type]env.ParserFunc{
			reflect.TypeFor[[]string](): func(v string) (any, error) {
				var s []string
				err := json.Unmarshal([]byte(v), &s)
				return s, err
			},
		},
	}
	if err := env.ParseWithOptions(&cfg, options); err != nil {
		log.Fatalf("Failed to parse environment variables: %v", err)
	}
	instance = &cfg
}

func GetConfig() *Config {
	return instance
}
