package vanilla

import (
	"github.com/gin-gonic/gin"
)

// PageInfo 分页信息
type PageInfo struct {
	Page         int
	FromId       int64
	CountPerPage int
	Mode         string // backend | apiserver
	Direction    string // asc | desc
}

// IsApiServerMode 是否为 API 服务端模式（游标分页）
func (p *PageInfo) IsApiServerMode() bool {
	return p.Mode == "apiserver"
}

// Desc 设置降序
func (p *PageInfo) Desc() *PageInfo {
	p.Direction = "desc"
	if p.FromId == 0 {
		p.FromId = 99999999999
	}
	return p
}

// Asc 设置升序
func (p *PageInfo) Asc() *PageInfo {
	p.Direction = "asc"
	return p
}

// ExtractPageInfoFromRequest 从 Gin 请求中抽取分页信息。
//
// backend 模式:  ?page=2&count_per_page=20
// apiserver 模式: ?_p_from=123&_p_count=20
func ExtractPageInfoFromRequest(c *gin.Context) *PageInfo {
	fromParam := c.Query("_p_from")
	if fromParam != "" {
		fromId, _ := parseQueryInt64(c, "_p_from")
		countPerPage, _ := parseQueryInt(c, "_p_count", 20)
		return &PageInfo{
			Page:         -1,
			FromId:       fromId,
			CountPerPage: countPerPage,
			Mode:         "apiserver",
			Direction:    "asc",
		}
	}
	page, _ := parseQueryInt(c, "page", 1)
	countPerPage, _ := parseQueryInt(c, "count_per_page", 20)
	return &PageInfo{
		Page:         page,
		FromId:       0,
		CountPerPage: countPerPage,
		Mode:         "backend",
		Direction:    "asc",
	}
}

func parseQueryInt(c *gin.Context, key string, def int) (int, error) {
	s := c.Query(key)
	if s == "" {
		return def, nil
	}
	v, err := parseParamInt(s)
	if err != nil {
		return def, nil
	}
	return int(v), nil
}

func parseQueryInt64(c *gin.Context, key string) (int64, error) {
	s := c.Query(key)
	if s == "" {
		return 0, nil
	}
	return parseParamInt(s)
}
