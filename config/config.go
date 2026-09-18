package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Load 加载配置文件到 out 指向的结构体。
//
// 加载顺序：
//  1. 先 AutomaticEnv + 设置 key replacer（保证环境变量读取生效）
//  2. 再读 GO_ENV 决定加载 config.<env>.yaml
//  3. 基础文件 config.yaml → 环境文件覆盖
//  4. BindEnv 敏感字段（db.dsn / redis.address / redis.password / auth.jwt_secret）
//  5. Unmarshal 到 out
//
// dir 为配置文件目录，通常为 "./conf"。
func Load(dir string, out any) error {
	v := viper.New()

	// 环境变量优先（必须在读取任何配置之前启用）
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 敏感字段显式绑定，支持 DB_DSN / REDIS_ADDRESS 等环境变量覆盖
	_ = v.BindEnv("db.dsn")
	_ = v.BindEnv("redis.address")
	_ = v.BindEnv("redis.password")
	_ = v.BindEnv("auth.jwt_secret")

	v.SetConfigType("yaml")
	v.AddConfigPath(dir)

	// 基础配置
	v.SetConfigName("config")
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("config: read base config: %w", err)
	}

	// 环境覆盖：GO_ENV=prod → config.prod.yaml
	goEnv := v.GetString("GO_ENV")
	if goEnv == "" {
		goEnv = os.Getenv("GO_ENV")
	}
	if goEnv != "" {
		v.SetConfigName("config." + goEnv)
		if err := v.MergeInConfig(); err != nil {
			// 环境配置文件不存在时容忍，仅打印提示
			fmt.Printf("config: no env override file config.%s.yaml, using base only\n", goEnv)
		}
	}

	if err := v.Unmarshal(out); err != nil {
		return fmt.Errorf("config: unmarshal: %w", err)
	}
	return nil
}
