// Package model 定义扫表得到的元数据结构，对应 Java 版 ColumnMetadata/FieldMetadata/TableMetadata。
package model

import (
	"fmt"
	"strings"
)

// ColumnMetadata 是扫表直接得到的列信息（数据库视角）。
type ColumnMetadata struct {
	ColumnName    string
	ColumnType    string
	ColumnComment string
	IsPrimaryKey  bool
}

// FieldMetadata 是经类型映射 + 命名转换后的字段（Java 视角）。
// 字段首字母大写 = 导出，模板（text/template）只能访问导出字段。
type FieldMetadata struct {
	JavaType     string
	JdbcType     string
	Name         string // 小驼峰字段名
	TableField   string // 原始列名
	Comment      string
	AutoFill     string // "" / insert / update / insertUpdate
	IsPrimaryKey bool
}

// TableMetadata 是一张表的完整元数据。
type TableMetadata struct {
	TableName    string
	TableComment string
	Columns      []ColumnMetadata
}

// FindSinglePrimaryKey 定位唯一主键；无主键或复合主键时快速失败。
// Go 小白知识点：Go 没有异常，错误通过返回值 error 显式传播（对应 Java 的 throw IllegalStateException）。
func FindSinglePrimaryKey(fields []FieldMetadata, tableName string) (FieldMetadata, error) {
	var pks []FieldMetadata
	for _, f := range fields {
		if f.IsPrimaryKey {
			pks = append(pks, f)
		}
	}
	if len(pks) == 0 {
		return FieldMetadata{}, fmt.Errorf("表 [%s] 无主键，无法生成代码（@TableId 需单列主键）", tableName)
	}
	if len(pks) > 1 {
		var cols []string
		for _, f := range pks {
			cols = append(cols, f.TableField)
		}
		return FieldMetadata{}, fmt.Errorf("表 [%s] 为复合主键(列: %s)，框架仅支持单列主键", tableName, strings.Join(cols, ", "))
	}
	return pks[0], nil
}
