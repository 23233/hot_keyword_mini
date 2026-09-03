package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

// JSONInt64Array 自定义类型，用于将 []int64 存储为数据库中的 JSON 字符串
type JSONInt64Array []int64

// Value 实现 driver.Valuer 接口，用于将 JSONInt64Array 转换为数据库可识别的值
// 使用指针接收器以保持方法集的一致性
func (a JSONInt64Array) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "[]", nil
	}
	return json.Marshal(a)
}

// Scan 实现 sql.Scanner 接口，用于将从数据库读取的值转换为 JSONInt64Array
func (a *JSONInt64Array) Scan(value interface{}) error {
	if value == nil {
		*a = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New(fmt.Sprintf("Failed to unmarshal JSONB value: %v of type %T", value, value))
	}
	// 如果字节为空或 "null"，则设置为空切片
	if len(bytes) == 0 || string(bytes) == "null" {
		*a = nil
		return nil
	}
	return json.Unmarshal(bytes, a)
}
