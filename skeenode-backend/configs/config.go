package config

import (
	"os"
	"strconv"
)

type Config struct {
	DBHost            string
	DBPort            string
	DBUser            string
	DBPassword        string
	DBName            string
	RedisHost         string
	RedisPort         string
	EtcdEndpoints     []string
	SchedulerInterval string
	LeaderElectionTTL int
	APIPort           string
	AIServiceURL      string
	// Auth settings
	JWTSecret      string
	JWTIssuer      string
	AuthEnabled    bool
}

func LoadConfig() *Config {
	return &Config{
		DBHost:            getEnv("DB_HOST", "localhost"),
		DBPort:            getEnv("DB_PORT", "5432"),
		DBUser:            getEnv("DB_USER", "skeenode"),
		DBPassword:        getEnv("DB_PASSWORD", "password"),
		DBName:            getEnv("DB_NAME", "skeenode"),
		RedisHost:         getEnv("REDIS_HOST", "localhost"),
		RedisPort:         getEnv("REDIS_PORT", "6379"),
		EtcdEndpoints:     []string{getEnv("ETCD_ENDPOINTS", "localhost:2379")},
		SchedulerInterval: getEnv("SCHEDULER_INTERVAL", "10s"),
		LeaderElectionTTL: getEnvAsInt("LEADER_ELECTION_TTL", 15),
		APIPort:           getEnv("API_PORT", "8080"),
		AIServiceURL:      getEnv("AI_SERVICE_URL", "http://localhost:8000"),
		// Auth settings
		JWTSecret:   getEnv("JWT_SECRET", ""),
		JWTIssuer:   getEnv("JWT_ISSUER", "skeenode"),
		AuthEnabled: getEnvAsBool("AUTH_ENABLED", false),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return fallback
	}
	return valueStr == "true" || valueStr == "1" || valueStr == "yes"
}
