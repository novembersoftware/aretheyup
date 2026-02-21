package structs

type Config struct {
	Env                          string `env:"ENV" envDefault:"dev"`
	APIPort                      string `env:"API_PORT" envDefault:"8080"`
	DBDSN                        string `env:"DB_DSN"`
	RedisURL                     string `env:"REDIS_URL"`
	GlobalRateLimitMaxRequests   int64  `env:"GLOBAL_RATE_LIMIT_MAX_REQUESTS" envDefault:"600"`
	GlobalRateLimitWindowSeconds int    `env:"GLOBAL_RATE_LIMIT_WINDOW_SECONDS" envDefault:"60"`
	PublicRateLimitMaxRequests   int64  `env:"PUBLIC_RATE_LIMIT_MAX_REQUESTS" envDefault:"240"`
	PublicRateLimitWindowSeconds int    `env:"PUBLIC_RATE_LIMIT_WINDOW_SECONDS" envDefault:"60"`
	ReportRateLimitMaxRequests   int64  `env:"REPORT_RATE_LIMIT_MAX_REQUESTS" envDefault:"1"`
	ReportRateLimitWindowSeconds int    `env:"REPORT_RATE_LIMIT_WINDOW_SECONDS" envDefault:"1800"`
}
