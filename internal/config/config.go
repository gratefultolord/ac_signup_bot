package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AdminBotToken string
	BotToken      string
	DBUser        string
	DBPassword    string
	DBName        string
	DBHost        string
	DBPort        string
}

func Load() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		log.Printf("config.Load: no .env file found - using env variables")
	}

	cfg := &Config{
		AdminBotToken: os.Getenv("ADMIN_BOT_TOKEN"),
		BotToken:      os.Getenv("BOT_TOKEN"),
		DBUser:        os.Getenv("DB_USER"),
		DBPassword:    os.Getenv("DB_PASSWORD"),
		DBName:        os.Getenv("DB_NAME"),
		DBHost:        os.Getenv("DB_HOST"),
		DBPort:        os.Getenv("DB_PORT"),
	}

	if cfg.AdminBotToken == "" {
		return nil, fmt.Errorf("config.Load: ADMIN_BOT_TOKEN is required")
	}

	if cfg.BotToken == "" {
		return nil, fmt.Errorf("config.Load: BOT_TOKEN is required")
	}

	if cfg.DBUser == "" || cfg.DBPassword == "" || cfg.DBName == "" {
		return nil, fmt.Errorf("config.Load: DB_USER, DB_PASSWORD, DB_NAME are required")
	}

	if cfg.DBHost == "" {
		cfg.DBHost = "localhost"
	}

	if cfg.DBPort == "" {
		cfg.DBPort = "5432"
	}

	return cfg, nil
}
