package db

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// ApplyFilters 将 vanilla 风格的 __f-字段-操作符 过滤器转换为 GORM Where 条件。
//
// 输入示例：
//
//	{
//	    "name": "张三",              → WHERE name = '张三'
//	    "__f-age-gt": "18",         → WHERE age > 18
//	    "__f-name-contain": "张",   → WHERE name LIKE '%张%'
//	    "__f-status-in": ["1","2"], → WHERE status IN ('1','2')
//	    "__f-created_at-range": ["2024-01-01","2024-12-31"] → WHERE created_at >= '2024-01-01' AND created_at <= '2024-12-31'
//	}
func ApplyFilters(db *gorm.DB, filters map[string]interface{}) *gorm.DB {
	for key, value := range filters {
		if !strings.HasPrefix(key, "__f") {
			db = db.Where(fmt.Sprintf("%s = ?", key), value)
			continue
		}
		// 解析 __f-field-op
		parts := strings.SplitN(key, "-", 3)
		if len(parts) < 3 {
			db = db.Where(fmt.Sprintf("%s = ?", strings.TrimPrefix(key, "__f-")), value)
			continue
		}
		field := parts[1]
		op := parts[2]
		switch op {
		case "equal":
			db = db.Where(fmt.Sprintf("%s = ?", field), value)
		case "contain":
			db = db.Where(fmt.Sprintf("%s LIKE ?", field), "%"+fmt.Sprintf("%v", value)+"%")
		case "gt":
			db = db.Where(fmt.Sprintf("%s > ?", field), value)
		case "gte":
			db = db.Where(fmt.Sprintf("%s >= ?", field), value)
		case "lt":
			db = db.Where(fmt.Sprintf("%s < ?", field), value)
		case "lte":
			db = db.Where(fmt.Sprintf("%s <= ?", field), value)
		case "in":
			db = db.Where(fmt.Sprintf("%s IN ?", field), value)
		case "notin":
			db = db.Where(fmt.Sprintf("%s NOT IN ?", field), value)
		case "range":
			if arr, ok := value.([]interface{}); ok && len(arr) >= 2 {
				db = db.Where(fmt.Sprintf("%s >= ? AND %s <= ?", field, field), arr[0], arr[1])
			}
		default:
			db = db.Where(fmt.Sprintf("%s = ?", field), value)
		}
	}
	return db
}
