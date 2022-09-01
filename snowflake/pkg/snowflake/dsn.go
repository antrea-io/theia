package snowflake

import (
	"log"
	"os"
	"strconv"

	sf "github.com/snowflakedb/gosnowflake"
)

func SetWarehouse(name string) func(*sf.Config) {
	return func(cfg *sf.Config) {
		cfg.Warehouse = name
	}
}

func SetDatabase(name string) func(*sf.Config) {
	return func(cfg *sf.Config) {
		cfg.Database = name
	}
}

func SetSchema(name string) func(*sf.Config) {
	return func(cfg *sf.Config) {
		cfg.Schema = name
	}
}

// GetDSN constructs a DSN based on the test connection parameters
func GetDSN(options ...func(*sf.Config)) (string, *sf.Config, error) {
	env := func(k string, failOnMissing bool) string {
		if value := os.Getenv(k); value != "" {
			return value
		}
		if failOnMissing {
			log.Fatalf("%v environment variable is not set.", k)
		}
		return ""
	}

	account := env("SNOWFLAKE_ACCOUNT", true)
	user := env("SNOWFLAKE_USER", true)
	password := env("SNOWFLAKE_PASSWORD", true)
	host := env("SNOWFLAKE_HOST", false)
	portStr := env("SNOWFLAKE_PORT", false)
	protocol := env("SNOWFLAKE_PROTOCOL", false)

	port := 443 // snowflake default port
	var err error
	if len(portStr) > 0 {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return "", nil, err
		}
	}

	cfg := &sf.Config{
		Account:  account,
		User:     user,
		Password: password,
		Host:     host,
		Port:     port,
		Protocol: protocol,
	}

	for _, fn := range options {
		fn(cfg)
	}

	dsn, err := sf.DSN(cfg)
	return dsn, cfg, err
}
