package main

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Database           DatabaseConfig `toml:"database"`
	Token              string         `toml:"token"`
	UseMembersIntent   bool           `toml:"use_members_intent"`
	LogLevel           slog.Level     `toml:"log_level"`
	NoPanic            bool           `toml:"no_panic"`
	DataUpdateInterval time.Duration  `toml:"data_update_interval"`
	GoogleAPIKey       string         `toml:"google_api_key"`
}

type DatabaseConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	Database string `toml:"database"`
	SSLMode  string `toml:"ssl_mode"`

	// used to set connection string, ignoring
	// the other properties
	urlOverride *url.URL `toml:"-"`
}

func (c *DatabaseConfig) ConnectionURL() url.URL {
	if c.urlOverride != nil {
		return *c.urlOverride
	}

	return url.URL{
		Scheme:   "postgres",
		Host:     c.Host + fmt.Sprintf(":%d", c.Port),
		User:     url.UserPassword(c.User, c.Password),
		Path:     c.Database,
		RawQuery: url.Values{"sslmode": []string{c.SSLMode}}.Encode(),
	}
}

func (c *DatabaseConfig) ConnectionString() string {
	u := c.ConnectionURL()
	return u.String()
}

func NewConfig() *Config {
	return &Config{}
}

func (c *Config) LoadDefaults() {
	if c.DataUpdateInterval.Abs() == 0 {
		c.DataUpdateInterval = 7 * 24 * time.Hour
	}

	if c.Database.SSLMode == "" {
		c.Database.SSLMode = "disable"
	}
}

func (c *Config) LoadEnv() error {
	tokenFile, ok := os.LookupEnv("BOTSU_TOKEN_FILE")

	if ok {
		token, err := os.ReadFile(tokenFile)
		if err != nil {
			return err
		}

		c.Token = string(token)
	}

	token, ok := os.LookupEnv("BOTSU_TOKEN")
	if ok {
		c.Token = token
	}

	host, ok := os.LookupEnv("POSTGRES_HOST")
	if ok {
		c.Database.Host = host
	}

	port, ok := os.LookupEnv("POSTGRES_PORT")
	if ok {
		portInt, err := strconv.Atoi(port)
		if err != nil {
			return err
		}

		c.Database.Port = portInt
	}

	userFile, ok := os.LookupEnv("POSTGRES_USER_FILE")
	if ok {
		user, err := os.ReadFile(userFile)
		if err != nil {
			return err
		}

		c.Database.User = string(user)
	}

	passwordFile, ok := os.LookupEnv("POSTGRES_PASSWORD_FILE")
	if ok {
		password, err := os.ReadFile(passwordFile)
		if err != nil {
			return err
		}

		c.Database.Password = string(password)
	}

	database, ok := os.LookupEnv("POSTGRES_DB")
	if ok {
		c.Database.Database = database
	}

	sslMode, ok := os.LookupEnv("POSTGRES_SSL_MODE")
	if ok {
		c.Database.SSLMode = sslMode
	}

	connectionString, ok := os.LookupEnv("BOTSU_CONNECTION_STRING")
	if ok {
		connectionURL, err := url.Parse(connectionString)
		if err != nil {
			return err
		}

		c.Database.urlOverride = connectionURL
	}

	useMembersIntent, ok := os.LookupEnv("BOTSU_USE_MEMBERS_INTENT")
	if ok {
		c.UseMembersIntent = stringToTruthy(useMembersIntent)
	}

	logLevel, ok := os.LookupEnv("BOTSU_LOG_LEVEL")
	if ok {
		if err := c.LogLevel.UnmarshalText([]byte(logLevel)); err != nil {
			return err
		}
	}

	noPanic, ok := os.LookupEnv("BOTSU_NO_PANIC")
	if ok {
		c.NoPanic = stringToTruthy(noPanic)
	}

	dataUpdateInterval, ok := os.LookupEnv("BOTSU_DATA_UPDATE_INTERVAL")
	if ok {
		duration, err := time.ParseDuration(dataUpdateInterval)
		if err != nil {
			return err
		}

		c.DataUpdateInterval = duration
	}

	googleAPIKey, ok := os.LookupEnv("BOTSU_GOOGLE_API_KEY")
	if ok {
		c.GoogleAPIKey = googleAPIKey
	}

	return nil
}

func (c *Config) Load(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	defer file.Close()

	err = toml.NewDecoder(file).Decode(c)
	if err != nil {
		return err
	}

	return nil
}

func stringToTruthy(s string) bool {
	switch strings.ToLower(s) {
	case "true", "t", "1", "yes", "y":
		return true
	default:
		return false
	}
}
