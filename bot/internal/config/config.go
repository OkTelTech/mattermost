package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                    string
	Env                     string
	BotURL                  string
	MongoURI                string
	MongoDB                 string
	MattermostURL           string
	AttendanceBotToken      string
	BudgetBotToken          string
	BlockMobile             bool
	ActivityCheckPeriodSec   int
	ActivityCheckTimeoutSec  int
	ActivityCheckIntervalSec int
}

func Load() *Config {
	return &Config{
		Port:                    getEnv("PORT", "3000"),
		Env:                     getEnv("ENV", "development"),
		BotURL:                  getEnv("BOT_URL", "http://bot-service:3000"),
		MongoURI:                getEnv("MONGODB_URI", "mongodb://localhost:27017"),
		MongoDB:                 getEnv("MONGODB_DATABASE", "oktel"),
		MattermostURL:           strings.TrimRight(getEnv("MATTERMOST_URL", "http://localhost:8065"), "/"),
		AttendanceBotToken:      getEnv("ATTENDANCE_BOT_TOKEN", ""),
		BudgetBotToken:          getEnv("BUDGET_BOT_TOKEN", ""),
		BlockMobile:             getEnv("ATTENDANCE_BLOCK_MOBILE", "true") == "true",
		ActivityCheckPeriodSec:   getEnvInt("ACTIVITY_CHECK_PERIOD", 3600),
		ActivityCheckTimeoutSec:  getEnvInt("ACTIVITY_CHECK_TIMEOUT", 10),
		ActivityCheckIntervalSec: getEnvInt("ACTIVITY_CHECK_INTERVAL", 300),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
