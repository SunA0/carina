package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/joho/godotenv"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

// ErrEnvFileNotFound 表示所有候选 env 文件均不存在。
// 该错误不致命：容器等纯环境变量注入场景下调用方打印提示后可继续。
var ErrEnvFileNotFound = errors.New("env file not found")

// LoadEnvFile 按约定加载 dotenv 文件到进程环境变量。
//
// 文件选择顺序：
//  1. explicit 非空 → 直接加载该文件
//  2. GO_ENV=dev|development → .env.development；prod|production → .env.production；
//     其他值 → .env.<GO_ENV 小写>
//  3. 回退 .env
//
// 已存在的进程环境变量优先于文件值（godotenv.Load 语义），
// 保证容器注入配置不被文件覆盖。
//
// 返回实际加载的文件名；全部候选缺失时返回包装了
// ErrEnvFileNotFound 的错误（调用方可用 errors.Is 判断后降级处理）。
func LoadEnvFile(explicit string) (string, error) {
	var candidates []string
	if explicit != "" {
		candidates = append(candidates, explicit)
	} else {
		if env := os.Getenv("GO_ENV"); env != "" {
			candidates = append(candidates, ".env."+normalizeEnv(env))
		}
		candidates = append(candidates, ".env")
	}

	for _, file := range candidates {
		if _, err := os.Stat(file); err != nil {
			continue
		}
		if err := godotenv.Load(file); err != nil {
			return "", fmt.Errorf("config: load env file %s: %w", file, err)
		}
		return file, nil
	}
	return "", fmt.Errorf("config: %w: tried %s", ErrEnvFileNotFound, strings.Join(candidates, ", "))
}

// normalizeEnv 将 GO_ENV 简写归一化为 env 文件名后缀。
func normalizeEnv(env string) string {
	switch strings.ToLower(env) {
	case "dev", "development":
		return "development"
	case "prod", "production":
		return "production"
	default:
		return strings.ToLower(env)
	}
}

// Load 从进程环境变量加载配置到 out 指向的结构体（env-only，不读取任何 yaml）。
//
// 机制：
//  1. 反射遍历 out 的结构类型，依 mapstructure 标签生成完整 dotted key 路径
//  2. 对每个叶子 key 执行 BindEnv，环境变量名 = key 大写且 "." → "_"
//     （如 server.cors_origins → SERVER_CORS_ORIGINS、db.max_idle → DB_MAX_IDLE）
//  3. Unmarshal 时启用 CSV decode hook，[]string 字段支持逗号分隔值
//
// out 必须为非 nil 指针。
func Load(out any) error {
	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("config: Load requires a non-nil pointer")
	}

	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bindEnvKeys(v, rv.Type().Elem(), "")

	if err := v.Unmarshal(out, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToSliceHookFunc(","),
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.TextUnmarshallerHookFunc(),
	))); err != nil {
		return fmt.Errorf("config: unmarshal: %w", err)
	}
	return nil
}

// bindEnvKeys 递归遍历结构体字段，为每个叶子 key 绑定环境变量。
func bindEnvKeys(v *viper.Viper, t reflect.Type, prefix string) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" { // 非导出字段
			continue
		}

		name, squash := parseMapstructureTag(f.Tag.Get("mapstructure"))
		if name == "-" {
			continue
		}

		key := prefix
		if name != "" {
			if key != "" {
				key += "."
			}
			key += name
		}

		ft := f.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}

		switch {
		case ft.Kind() == reflect.Struct && squash:
			// 内嵌结构（如 config.Base），字段提升到父级路径
			bindEnvKeys(v, ft, prefix)
		case ft.Kind() == reflect.Struct:
			bindEnvKeys(v, ft, key)
		default:
			if key != "" && isLeafKind(ft.Kind()) {
				_ = v.BindEnv(key)
			}
		}
	}
}

// parseMapstructureTag 解析 mapstructure 标签，返回字段名与是否 squash。
func parseMapstructureTag(tag string) (name string, squash bool) {
	parts := strings.Split(tag, ",")
	name = parts[0]
	for _, opt := range parts[1:] {
		if opt == "squash" {
			squash = true
		}
	}
	return name, squash
}

// isLeafKind 判断字段类型是否作为配置叶子处理（标量及切片/映射，含 []string）。
func isLeafKind(k reflect.Kind) bool {
	switch k {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String,
		reflect.Slice, reflect.Array, reflect.Map:
		return true
	default:
		return false
	}
}
