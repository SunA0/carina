package config

// ServerConfig 服务配置
type ServerConfig struct {
	Host        string   `mapstructure:"host"`
	Port        int      `mapstructure:"port"`
	Mode        string   `mapstructure:"mode"` // debug, release, test
	CORSOrigins []string `mapstructure:"cors_origins"`
}

// DBConfig 数据库配置
type DBConfig struct {
	DSN     string `mapstructure:"dsn"`
	MaxIdle int    `mapstructure:"max_idle"`
	MaxOpen int    `mapstructure:"max_open"`
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	JWTSecret string `mapstructure:"jwt_secret"`
}

// TracingConfig 链路追踪配置
type TracingConfig struct {
	Enabled    bool    `mapstructure:"enabled"`
	Endpoint   string  `mapstructure:"endpoint"` // OTLP HTTP endpoint
	SampleRate float64 `mapstructure:"sample_rate"`
}

// LockConfig 分布式锁配置
type LockConfig struct {
	Engine       string `mapstructure:"engine"` // dummy | redis
	RedisAddress string `mapstructure:"redis_address"`
	RedisDB      int    `mapstructure:"redis_db"`
	RedisPasswd  string `mapstructure:"redis_password"`
}

// Base 框架通用配置基座，项目本地 config 内嵌此结构后追加业务字段
type Base struct {
	Server  ServerConfig  `mapstructure:"server"`
	DB      DBConfig      `mapstructure:"db"`
	Redis   RedisConfig   `mapstructure:"redis"`
	Auth    AuthConfig    `mapstructure:"auth"`
	Tracing TracingConfig `mapstructure:"tracing"`
	Lock    LockConfig    `mapstructure:"lock"`
}
