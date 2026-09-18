package vanilla

import (
	"fmt"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// RESOURCES 已注册的资源名集合
var RESOURCES = make([]string, 0, 100)

var enableDevTestResource = os.Getenv("ENABLE_DEV_TEST_RESOURCE") == "1"

// Router 将资源注册到 Gin engine，生成约定式 URL。
//
// 资源名 "account.corp" 生成以下路由：
//   - 标准 URL:  /account/corp/
//   - API URL:   /account/api/corp/
//   - 别名 URL:  通过 GetAlias() 自定义
func Router(engine *gin.Engine, r RestResourceInterface) {
	if r.IsForDevTest() && !enableDevTestResource {
		return
	}

	resource := r.Resource()
	RESOURCES = append(RESOURCES, resource)
	items := strings.Split(resource, ".")

	handler := CreateHandler(r)

	// 标准 URL: /account/corp/
	standardPath := "/" + strings.Join(items, "/") + "/"
	registerRoute(engine, standardPath, handler)

	// API URL: /account/api/corp/
	apiItems := make([]string, len(items))
	copy(apiItems, items)
	lastItem := items[len(items)-1]
	apiItems[len(apiItems)-1] = "api"
	apiPath := "/" + strings.Join(apiItems, "/") + "/" + lastItem + "/"
	registerRoute(engine, apiPath, handler)

	// 别名 URL
	for _, alias := range r.GetAlias() {
		aliasPath := alias
		if !strings.HasPrefix(aliasPath, "/") {
			aliasPath = "/" + aliasPath
		}
		if !strings.HasSuffix(aliasPath, "/") {
			aliasPath += "/"
		}
		registerRoute(engine, aliasPath, handler)
	}
}

func registerRoute(engine *gin.Engine, path string, handler gin.HandlerFunc) {
	engine.GET(path, handler)
	engine.POST(path, handler)
	engine.PUT(path, handler)
	engine.DELETE(path, handler)
	engine.PATCH(path, handler)
}

// RouterInfo 打印注册信息
func RouterInfo() string {
	return fmt.Sprintf("[carina] registered %d resources", len(RESOURCES))
}
