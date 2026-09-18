// Package config 项目本地配置：内嵌框架 Base，追加业务字段。
// 框架新增配置段时本结构自动获得，无需改动。
package config

import (
	carinaconfig "github.com/suna0/carina/config"
)

// Config 项目配置根结构
type Config struct {
	carinaconfig.Base `mapstructure:",squash"`

	// ---- 业务自定义字段 ----
	AppName  string `mapstructure:"app_name"`
	Greeting string `mapstructure:"greeting"`
}
