package structs

type Config struct {
	Env      string `env:"ENV" envDefault:"dev"`
	APIPort  string `env:"API_PORT" envDefault:"8080"`
	DBDSN    string `env:"DB_DSN"`
	RedisURL string `env:"REDIS_URL"`
}
