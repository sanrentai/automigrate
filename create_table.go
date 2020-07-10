package automigrate

import (
	"fmt"
	"reflect"
	"strings"
)

// 创建关系表
func (scope *Scope) createJoinTable(field *StructField) {
	if relationship := field.Relationship; relationship != nil && relationship.JoinTableHandler != nil {
		joinTableHandler := relationship.JoinTableHandler
		joinTable := joinTableHandler.Table(scope.db)
		if !scope.Dialect().HasTable(joinTable) {
			toScope := &Scope{Value: reflect.New(field.Struct.Type).Interface()}

			var sqlTypes, primaryKeys []string
			for idx, fieldName := range relationship.ForeignFieldNames {
				if field, ok := scope.FieldByName(fieldName); ok {
					foreignKeyStruct := field.clone()
					foreignKeyStruct.IsPrimaryKey = false
					foreignKeyStruct.TagSettingsSet("IS_JOINTABLE_FOREIGNKEY", "true")
					foreignKeyStruct.TagSettingsDelete("AUTO_INCREMENT")
					sqlTypes = append(sqlTypes, scope.Quote(relationship.ForeignDBNames[idx])+" "+scope.Dialect().DataTypeOf(foreignKeyStruct))
					primaryKeys = append(primaryKeys, scope.Quote(relationship.ForeignDBNames[idx]))
				}
			}

			for idx, fieldName := range relationship.AssociationForeignFieldNames {
				if field, ok := toScope.FieldByName(fieldName); ok {
					foreignKeyStruct := field.clone()
					foreignKeyStruct.IsPrimaryKey = false
					foreignKeyStruct.TagSettingsSet("IS_JOINTABLE_FOREIGNKEY", "true")
					foreignKeyStruct.TagSettingsDelete("AUTO_INCREMENT")
					sqlTypes = append(sqlTypes, scope.Quote(relationship.AssociationForeignDBNames[idx])+" "+scope.Dialect().DataTypeOf(foreignKeyStruct))
					primaryKeys = append(primaryKeys, scope.Quote(relationship.AssociationForeignDBNames[idx]))
				}
			}

			scope.Err(scope.NewDB().Exec(fmt.Sprintf("CREATE TABLE %v (%v, PRIMARY KEY (%v))%s", scope.Quote(joinTable), strings.Join(sqlTypes, ","), strings.Join(primaryKeys, ","), scope.getTableOptions())).Error)
		}
		scope.NewDB().Table(joinTable).AutoMigrate(joinTableHandler)
	}
}

func (scope *Scope) createTable() *Scope {
	var tags []string
	var primaryKeys []string
	var primaryKeyInColumnType = false
	for _, field := range scope.GetModelStruct().StructFields {
		if field.IsNormal {
			sqlTag := scope.Dialect().DataTypeOf(field)

			// Check if the primary key constraint was specified as
			// part of the column type. If so, we can only support
			// one column as the primary key.
			if strings.Contains(strings.ToLower(sqlTag), "primary key") {
				primaryKeyInColumnType = true
			}

			tags = append(tags, scope.Quote(field.DBName)+" "+sqlTag)
		}

		if field.IsPrimaryKey {
			primaryKeys = append(primaryKeys, scope.Quote(field.DBName))
		}
		scope.createJoinTable(field)
	}

	var primaryKeyStr string
	if len(primaryKeys) > 0 && !primaryKeyInColumnType {
		primaryKeyStr = fmt.Sprintf(", PRIMARY KEY (%v)", strings.Join(primaryKeys, ","))
	}

	scope.Raw(fmt.Sprintf("CREATE TABLE %v (%v %v)%s", scope.QuotedTableName(), strings.Join(tags, ","), primaryKeyStr, scope.getTableOptions())).Exec()

	scope.autoIndex()
	return scope
}
