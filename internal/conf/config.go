package conf

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    *RedisConfig   `mapstructure:"redis"` // 指针类型，可选
	Log      LogConfig      `mapstructure:"log"`
	App      AppConfig      `mapstructure:"app"`
}

type ServerConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Mode         string `mapstructure:"mode"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
}

type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"db_name"`
	SSLMode  string `mapstructure:"ssl_mode"`
}

func (c *DatabaseConfig) DSN() string {
	switch c.Driver {
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			c.User, c.Password, c.Host, c.Port, c.DBName)
	case "postgres":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
	case "sqlite":
		return c.DBName
	default:
		return ""
	}
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

func (c *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type LogConfig struct {
	Level      string `mapstructure:"level"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
}

var GlobalConfig *Config

// envKeyReplacer 让 viper 的键（如 database.password）能匹配上
// 下划线形态的环境变量（DATABASE_PASSWORD）。
var envKeyReplacer = strings.NewReplacer(".", "_")

func Init(configPath string) error {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	v := newEnvAwareViper()
	if err := v.ReadConfig(bytes.NewReader(expandEnvRefs(raw))); err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	return decode(v)
}

func InitWithDefault() error {
	v := newEnvAwareViper()

	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8888)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.read_timeout", 30)
	v.SetDefault("server.write_timeout", 30)
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.db_name", "./data/app.db")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.max_size", 100)
	v.SetDefault("log.max_backups", 10)
	v.SetDefault("log.max_age", 30)
	v.SetDefault("log.compress", true)
	v.SetDefault("app.name", "template")
	v.SetDefault("app.version", "1.0.0")

	return decode(v)
}

// newEnvAwareViper 构造接好环境变量覆盖链的 viper 实例：
// 环境变量（最高）> 配置文件 > 代码默认值。
// 每次使用独立实例，避免包级 viper 全局状态在测试/多次 Init 间互相污染。
func newEnvAwareViper() *viper.Viper {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvKeyReplacer(envKeyReplacer)
	v.AutomaticEnv()
	return v
}

// decode 绑定全部已知配置键的环境变量并解出 GlobalConfig。
// viper.Unmarshal 基于 AllKeys()，AutomaticEnv 的键不在其中，
// 所以必须显式 BindEnv：YAML 里没出现、只在环境变量里给的配置项
// （如 DATABASE_SSL_MODE）才能进得来。
func decode(v *viper.Viper) error {
	bindStructEnvs(v, "", &Config{})

	// 文档（README/REDIS.md）承诺的短名别名
	_ = v.BindEnv("database.password", "DB_PASSWORD")
	_ = v.BindEnv("redis.password", "REDIS_PASSWORD")

	GlobalConfig = &Config{}
	if err := v.Unmarshal(GlobalConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// bindStructEnvs 按 mapstructure 标签递归遍历配置结构体，
// 为每个叶子键 BindEnv，保证「YAML 未出现的键也能从环境变量取值」。
// in 必须传入可寻址的结构体（指针或其字段地址）。
func bindStructEnvs(v *viper.Viper, prefix string, in any) {
	rv := reflect.Indirect(reflect.ValueOf(in))
	if rv.Kind() != reflect.Struct {
		return
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		tag := f.Tag.Get("mapstructure")
		if tag == "" {
			continue
		}
		key := tag
		if prefix != "" {
			key = prefix + "." + key
		}
		switch {
		case f.Type.Kind() == reflect.Struct:
			bindStructEnvs(v, key, rv.Field(i).Addr().Interface())
		case f.Type.Kind() == reflect.Pointer && f.Type.Elem().Kind() == reflect.Struct:
			bindStructEnvs(v, key, reflect.New(f.Type.Elem()).Interface())
		default:
			_ = v.BindEnv(key)
		}
	}
}

// expandEnvRefs 展开 YAML 中的 ${VAR} / $VAR 环境变量引用
// （README「生产配置建议」与 REDIS.md 承诺的 ${DB_PASSWORD} 写法）。
// 未定义的引用原样保留，避免值被静默置空；$$、$1 等 shell 特殊语法不受影响。
func expandEnvRefs(data []byte) []byte {
	return []byte(os.Expand(string(data), func(name string) string {
		if isShellSpecialVar(name) {
			return "$" + name
		}
		if val, ok := os.LookupEnv(name); ok {
			return val
		}
		return "${" + name + "}"
	}))
}

func isShellSpecialVar(name string) bool {
	return len(name) == 1 && strings.ContainsAny(name, "*#$@!?-0123456789")
}
