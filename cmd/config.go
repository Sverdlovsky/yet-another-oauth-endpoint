package main

import (
	"fmt"
	"os"
	"time"
)

const jwtExpiration = 12 * time.Hour

type Config struct {
	SecretKey string
	JWTSecret []byte
	Domain    string

	YandexClientID, YandexClientSecret string
	GoogleClientID, GoogleClientSecret string
	GithubClientID, GithubClientSecret string
}

func requireEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		fmt.Fprintf(os.Stderr, "missing required env var: %s\n", name)
		os.Exit(1)
	}
	return v
}

func loadConfig() *Config {
	return &Config{
		SecretKey: requireEnv("SECRET_KEY"),
		JWTSecret: []byte(requireEnv("JWT_SECRET")),
		Domain:    requireEnv("DOMAIN"),

		YandexClientID:     os.Getenv("YANDEX_CLIENT_ID"),
		YandexClientSecret: os.Getenv("YANDEX_CLIENT_SECRET"),

		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),

		GithubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GithubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
	}
}

