package vanilla

import (
	"strconv"
)

// parseInt 解析 int
func parseInt(s string, result *int) (int, error) {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	*result = v
	return v, nil
}

// parseInt64 解析 int64
func parseInt64(s string, result *int64) (int64, error) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	*result = v
	return v, nil
}

// parseFloat 解析 float64
func parseFloat(s string, result *float64) (float64, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	*result = v
	return v, nil
}

// parseBool 解析 bool
func parseBool(s string) bool {
	v, _ := strconv.ParseBool(s)
	return v
}
