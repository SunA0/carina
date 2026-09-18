// Package notify 提供告警通知能力（钉钉机器人）。
// 移植自 vanilla ding_bot，去除 beego 依赖：配置通过 Options 显式传入。
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const defaultAPIURL = "https://oapi.dingtalk.com/robot/send"

// Options 钉钉机器人配置
type Options struct {
	// Token 机器人 access_token，为空时仅打印日志不发送
	Token string
	// BotName 机器人名称（仅作为标识）
	BotName string
	// Mode 运行环境：develop / test / deploy
	// develop 模式下只打印不发送
	Mode string
	// PlatformName 平台名称（附加在消息尾部）
	PlatformName string
	// APIURL 自定义 API 地址，默认为钉钉官方地址
	APIURL string
	// Timeout HTTP 请求超时，默认 5s
	Timeout time.Duration
}

// DingBot 钉钉机器人客户端
type DingBot struct {
	opts   Options
	client *http.Client
}

// NewDingBot 创建钉钉机器人
func NewDingBot(opts Options) *DingBot {
	if opts.APIURL == "" {
		opts.APIURL = defaultAPIURL
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Second
	}
	return &DingBot{
		opts:   opts,
		client: &http.Client{Timeout: opts.Timeout},
	}
}

// envName 环境中文名
func (b *DingBot) envName() string {
	if cn, ok := map[string]string{
		"develop": "开发环境",
		"test":    "测试环境",
		"deploy":  "生产环境",
	}[b.opts.Mode]; ok {
		return cn
	}
	return "未知环境"
}

// Send 发送 markdown 消息
//
//	{
//	    "msgtype": "markdown",
//	    "markdown": {"title": "...", "text": "..."}
//	}
func (b *DingBot) Send(ctx context.Context, title, msg string) error {
	if b.opts.Token == "" || b.opts.Mode == "develop" {
		// 开发环境或 token 不存在则只打印
		log.Printf("[dingbot] %s %s", title, msg)
		return nil
	}

	apiURL := fmt.Sprintf("%s?access_token=%s", b.opts.APIURL, b.opts.Token)

	data := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  fmt.Sprintf("%s \n > %s%s %s", msg, b.opts.PlatformName, b.envName(), title),
		},
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("dingbot: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("dingbot: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("dingbot: send: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dingbot: unexpected status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// Info 通知
func (b *DingBot) Info(ctx context.Context, msg string) error {
	return b.Send(ctx, "通知...", msg)
}

// Warn 警告
func (b *DingBot) Warn(ctx context.Context, msg string) error {
	return b.Send(ctx, "警告-_-", msg)
}

// Error 错误
func (b *DingBot) Error(ctx context.Context, msg string) error {
	return b.Send(ctx, "错误>_<", msg)
}

// Critical 严重错误
func (b *DingBot) Critical(ctx context.Context, msg string) error {
	return b.Send(ctx, "严重错误！！！", msg)
}
