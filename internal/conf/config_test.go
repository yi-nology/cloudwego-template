package conf

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写配置文件失败: %v", err)
	}
	return path
}

// 回归 #2：${DB_PASSWORD} 占位符必须展开（README/REDIS.md 承诺的写法）。
func TestInitExpandsEnvPlaceholder(t *testing.T) {
	t.Setenv("DB_PASSWORD", "real-db-secret")
	path := writeConfig(t, `
database:
  driver: "postgres"
  password: "${DB_PASSWORD}"
`)
	if err := Init(path); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
	if got := GlobalConfig.Database.Password; got != "real-db-secret" {
		t.Fatalf("database.password = %q, want real-db-secret（占位符未展开）", got)
	}
}

// 回归 #2：未设置的环境变量引用原样保留，不被静默置空。
func TestInitKeepsUndefinedPlaceholder(t *testing.T) {
	t.Setenv("DB_PASSWORD", "")
	os.Unsetenv("DB_PASSWORD")
	path := writeConfig(t, `
database:
  password: "${DB_PASSWORD}"
`)
	if err := Init(path); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
	if got := GlobalConfig.Database.Password; got != "${DB_PASSWORD}" {
		t.Fatalf("database.password = %q, want 未展开的字面量 ${DB_PASSWORD}", got)
	}
}

// 回归 #2：DB_PASSWORD 短名（文档示例）必须能覆盖 database.password。
func TestInitDBPasswordShortNameOverrides(t *testing.T) {
	t.Setenv("DB_PASSWORD", "doc-alias-secret")
	path := writeConfig(t, `
database:
  password: "yaml-plain-secret"
`)
	if err := Init(path); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
	if got := GlobalConfig.Database.Password; got != "doc-alias-secret" {
		t.Fatalf("database.password = %q, want doc-alias-secret（DB_PASSWORD 未生效）", got)
	}
}

// 回归 #2：下划线形态 DATABASE_PASSWORD（键名 replacer）也能覆盖。
func TestInitEnvKeyReplacerOverrides(t *testing.T) {
	t.Setenv("DATABASE_PASSWORD", "replacer-secret")
	path := writeConfig(t, `
database:
  password: "yaml-plain-secret"
`)
	if err := Init(path); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
	if got := GlobalConfig.Database.Password; got != "replacer-secret" {
		t.Fatalf("database.password = %q, want replacer-secret", got)
	}
}

// 回归 #2：YAML 里没有的键也能从环境变量进来（viper.Unmarshal 不吃
// AutomaticEnv 的键，必须显式 BindEnv）。
func TestInitEnvOnlyKeyFromStruct(t *testing.T) {
	t.Setenv("DATABASE_SSL_MODE", "require")
	path := writeConfig(t, `
database:
  driver: "postgres"
  host: "localhost"
`)
	if err := Init(path); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
	if got := GlobalConfig.Database.SSLMode; got != "require" {
		t.Fatalf("database.ssl_mode = %q, want require", got)
	}
}

// 回归 #2：REDIS_PASSWORD（REDIS.md 示例）覆盖 redis.password。
func TestInitRedisPasswordOverride(t *testing.T) {
	t.Setenv("REDIS_PASSWORD", "redis-secret")
	path := writeConfig(t, `
redis:
  host: "localhost"
  password: "${REDIS_PASSWORD}"
`)
	if err := Init(path); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
	if got := GlobalConfig.Redis.Password; got != "redis-secret" {
		t.Fatalf("redis.password = %q, want redis-secret", got)
	}
}

// $$ 等 shell 特殊语法不受展开影响。
func TestInitPreservesShellSpecialSyntax(t *testing.T) {
	path := writeConfig(t, `
database:
  password: "p$$w"
`)
	if err := Init(path); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
	if got := GlobalConfig.Database.Password; got != "p$$w" {
		t.Fatalf("database.password = %q, want p$$w", got)
	}
}

// InitWithDefault 同样要走环境变量覆盖链（bootstrap 的兜底路径）。
func TestInitWithDefaultEnvOverride(t *testing.T) {
	t.Setenv("DB_PASSWORD", "default-path-secret")
	if err := InitWithDefault(); err != nil {
		t.Fatalf("InitWithDefault 失败: %v", err)
	}
	if got := GlobalConfig.Database.Password; got != "default-path-secret" {
		t.Fatalf("database.password = %q, want default-path-secret", got)
	}
	if got := GlobalConfig.Server.Port; got != 8888 {
		t.Fatalf("默认值应保留: server.port = %d, want 8888", got)
	}
}
