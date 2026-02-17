package structs

type Config struct {
	Env     string `env:"ENV" envDefault:"dev"`
	APIPort string `env:"API_PORT" envDefault:"8080"`
}
