package vanilla

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// parseParameters 根据 GetParameters() 声明解析并校验请求参数。
// 返回 false 表示校验失败（已返回错误响应）。
// embedded 为实例内嵌的 *RestResource，用于落盘 Name2JSON/Filters 等字段；
// 未内嵌基类（embedded == nil）时仍做类型校验，仅跳过字段落盘。
func parseParameters(c *gin.Context, embedded *RestResource, instance RestResourceInterface, method string) bool {
	parameters := instance.GetParameters()
	if parameters == nil {
		return true
	}

	params, ok := parameters[method]
	if !ok {
		return true
	}

	for _, param := range params {
		optional := false
		if strings.HasPrefix(param, "?") {
			optional = true
			param = param[1:]
		}

		parts := strings.SplitN(param, ":", 2)
		paramName := parts[0]
		paramType := "string"
		if len(parts) == 2 {
			paramType = parts[1]
		}

		value := paramValue(c, paramName)
		if value == "" {
			if !optional {
				returnValidateFail(c, paramName, paramType, "parameter is required")
				return false
			}
			continue
		}

		switch paramType {
		case "string":
			// 无需校验
		case "int":
			if _, err := parseParamInt(value); err != nil {
				returnValidateFail(c, paramName, paramType, err.Error())
				return false
			}
		case "float":
			if _, err := parseParamFloat(value); err != nil {
				returnValidateFail(c, paramName, paramType, err.Error())
				return false
			}
		case "bool":
			if !isValidBool(value) {
				returnValidateFail(c, paramName, paramType, "invalid bool value")
				return false
			}
		case "json":
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(value), &data); err != nil {
				returnValidateFail(c, paramName, paramType, err.Error())
				return false
			}
			if embedded != nil {
				if paramName == "filters" {
					embedded.Filters = data
				} else {
					embedded.Name2JSON[paramName] = data
				}
			}
		case "json-raw":
			var data interface{}
			if err := json.Unmarshal([]byte(value), &data); err != nil {
				returnValidateFail(c, paramName, paramType, err.Error())
				return false
			}
			if embedded != nil {
				embedded.Name2RAWJSON[paramName] = data
			}
		case "json-array":
			var data []interface{}
			if err := json.Unmarshal([]byte(value), &data); err != nil {
				returnValidateFail(c, paramName, paramType, err.Error())
				return false
			}
			if embedded != nil {
				embedded.Name2JSONArray[paramName] = data
			}
		}
	}

	// 解析 __f-xxx 过滤器
	if embedded != nil {
		for key := range c.Request.URL.Query() {
			if strings.HasPrefix(key, "__f") {
				parts := strings.SplitN(key, "-", 3)
				if len(parts) == 3 {
					op := parts[2]
					value := c.Query(key)
					switch op {
					case "in", "notin", "range":
						var arr []interface{}
						if err := json.Unmarshal([]byte(value), &arr); err == nil {
							embedded.Filters[key] = arr
						} else {
							embedded.Filters[key] = value
						}
					default:
						embedded.Filters[key] = value
					}
				}
			}
		}
	}

	return true
}

func returnValidateFail(c *gin.Context, param, paramType, innerMsg string) {
	c.JSON(http.StatusOK, MakeErrorResponse(500, "rest:missing_argument",
		fmt.Sprintf("missing or invalid argument: %s(%s)", param, paramType), innerMsg))
}

func parseParamInt(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

func parseParamFloat(s string) (float64, error) {
	var v float64
	_, err := fmt.Sscanf(s, "%g", &v)
	return v, err
}

func isValidBool(s string) bool {
	lower := strings.ToLower(s)
	return lower == "true" || lower == "false" || lower == "1" || lower == "0"
}
