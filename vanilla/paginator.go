package vanilla

import (
	"fmt"
	"reflect"

	"gorm.io/gorm"
)

// INextPageInfo 分页结果接口
type INextPageInfo interface {
	ToMap() map[string]interface{}
}

// PaginateResult 经典分页结果
type PaginateResult struct {
	HasPrev      bool
	HasNext      bool
	HasHead      bool
	HasTail      bool
	Prev         int
	Next         int
	CurPage      int
	MaxPage      int
	TotalCount   int64
	DisplayPages []int
	Offset       int
	CountInPage  int
}

// ToMap 转换为 map
func (r PaginateResult) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"has_head":      r.HasHead,
		"has_tail":      r.HasTail,
		"has_prev":      r.HasPrev,
		"has_next":      r.HasNext,
		"next":          r.Next,
		"prev":          r.Prev,
		"total_count":   r.TotalCount,
		"object_count":  r.TotalCount,
		"cur_page":      r.CurPage,
		"display_pages": r.DisplayPages,
		"max_page":      r.MaxPage,
	}
}

// APIServiceNextPageInfo 游标分页结果
type APIServiceNextPageInfo struct {
	HasNext    bool
	NextFromId int64
}

// ToMap 转换为 map
func (r APIServiceNextPageInfo) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"has_next":     r.HasNext,
		"next_from_id": r.NextFromId,
	}
}

// Paginate 执行分页查询。
//
// backend 模式:  COUNT + LIMIT/OFFSET，返回 PaginateResult
// apiserver 模式: LIMIT count+1 判断是否还有下一页，返回 APIServiceNextPageInfo
func Paginate(db *gorm.DB, page *PageInfo, container interface{}) (INextPageInfo, error) {
	if page.IsApiServerMode() {
		return paginateCursor(db, page, container)
	}
	return paginateOffset(db, page, container)
}

// paginateOffset 经典 offset/limit 分页
func paginateOffset(db *gorm.DB, page *PageInfo, container interface{}) (INextPageInfo, error) {
	var totalCount int64
	if err := db.Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("paginate: count: %w", err)
	}

	result := doPaginate(totalCount, page.Page, page.CountPerPage)

	offset := result.Offset
	limit := result.CountInPage
	if err := db.Offset(offset).Limit(limit).Find(container).Error; err != nil {
		return nil, fmt.Errorf("paginate: find: %w", err)
	}

	return result, nil
}

// paginateCursor 游标分页（多取 1 条判断 hasNext）
func paginateCursor(db *gorm.DB, page *PageInfo, container interface{}) (INextPageInfo, error) {
	// 多取 1 条用于判断
	query := db
	if page.Direction == "desc" {
		query = query.Where("id < ?", page.FromId).Order("id DESC")
	} else {
		query = query.Where("id > ?", page.FromId).Order("id ASC")
	}
	if err := query.Limit(page.CountPerPage + 1).Find(container).Error; err != nil {
		return nil, fmt.Errorf("paginate: cursor find: %w", err)
	}

	val := reflect.ValueOf(container)
	ind := reflect.Indirect(val)
	realCount := ind.Len()

	if realCount > page.CountPerPage {
		// 有下一页，截取倒数第 2 条的 ID
		lastItem := ind.Index(realCount - 2)
		var lastId int64
		if field := lastItem.FieldByName("Id"); field.IsValid() {
			lastId = field.Int()
		} else if field := lastItem.FieldByName("ID"); field.IsValid() {
			lastId = field.Int()
		}

		// 截取前 count 条
		slice := reflect.MakeSlice(ind.Type(), 0, 0)
		for i := 0; i < page.CountPerPage; i++ {
			slice = reflect.Append(slice, ind.Index(i))
		}
		ind.Set(slice)

		return &APIServiceNextPageInfo{
			HasNext:    true,
			NextFromId: lastId,
		}, nil
	}

	return &APIServiceNextPageInfo{
		HasNext:    false,
		NextFromId: -1,
	}, nil
}

// doPaginate 计算分页元数据
func doPaginate(itemCount int64, curPage int, itemCountPerPage int) *PaginateResult {
	result := &PaginateResult{TotalCount: itemCount}
	total := getTotalPageCount(itemCount, itemCountPerPage)

	if curPage > total {
		curPage = total
	}
	result.MaxPage = total
	result.CurPage = curPage

	result.HasTail = curPage < total
	result.HasHead = curPage > 1
	result.HasPrev = curPage > 1
	result.HasNext = curPage < total

	if result.HasPrev {
		result.Prev = curPage - 1
	}
	if result.HasNext {
		result.Next = curPage + 1
	}

	// 显示页码序列
	if result.MaxPage <= 5 {
		result.DisplayPages = intRange(1, result.MaxPage)
	} else if curPage+2 <= result.MaxPage {
		if curPage >= 3 {
			result.DisplayPages = intRange(curPage-2, curPage+2)
		} else {
			result.DisplayPages = intRange(1, 5)
		}
	} else {
		result.DisplayPages = intRange(result.MaxPage-4, result.MaxPage)
	}

	result.Offset = (curPage - 1) * itemCountPerPage
	result.CountInPage = itemCountPerPage

	return result
}

func getTotalPageCount(itemCount int64, perPage int) int {
	perPage64 := int64(perPage)
	if perPage64 == 0 {
		return 1
	}
	total := itemCount / perPage64
	if itemCount%perPage64 != 0 {
		total++
	}
	if total == 0 {
		total = 1
	}
	return int(total)
}

func intRange(start, end int) []int {
	if start > end {
		return []int{}
	}
	result := make([]int, 0, end-start+1)
	for i := start; i <= end; i++ {
		result = append(result, i)
	}
	return result
}
