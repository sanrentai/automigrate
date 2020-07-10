package automigrate

import (
	"fmt"
	"reflect"
	"strings"
)

// JoinTableHandlerInterface is an interface for how to handle many2many relations
type JoinTableHandlerInterface interface {
	// initialize join table handler
	Setup(relationship *Relationship, tableName string, source reflect.Type, destination reflect.Type)
	// Table return join table's table name
	Table(db *DB) string
	// Add create relationship in join table for source and destination
	Add(handler JoinTableHandlerInterface, db *DB, source interface{}, destination interface{}) error
	// SourceForeignKeys return source foreign keys
	SourceForeignKeys() []JoinTableForeignKey
	// DestinationForeignKeys return destination foreign keys
	DestinationForeignKeys() []JoinTableForeignKey
}

// JoinTableForeignKey join table foreign key struct
type JoinTableForeignKey struct {
	DBName            string
	AssociationDBName string
}

// JoinTableSource is a struct that contains model type and foreign keys
type JoinTableSource struct {
	ModelType   reflect.Type
	ForeignKeys []JoinTableForeignKey
}

// JoinTableHandler default join table handler
type JoinTableHandler struct {
	TableName   string          `sql:"-"`
	Source      JoinTableSource `sql:"-"`
	Destination JoinTableSource `sql:"-"`
}

// SourceForeignKeys return source foreign keys
func (s *JoinTableHandler) SourceForeignKeys() []JoinTableForeignKey {
	return s.Source.ForeignKeys
}

// DestinationForeignKeys return destination foreign keys
func (s *JoinTableHandler) DestinationForeignKeys() []JoinTableForeignKey {
	return s.Destination.ForeignKeys
}

// Setup initialize a default join table handler
func (s *JoinTableHandler) Setup(relationship *Relationship, tableName string, source reflect.Type, destination reflect.Type) {
	s.TableName = tableName

	s.Source = JoinTableSource{ModelType: source}
	s.Source.ForeignKeys = []JoinTableForeignKey{}
	for idx, dbName := range relationship.ForeignFieldNames {
		s.Source.ForeignKeys = append(s.Source.ForeignKeys, JoinTableForeignKey{
			DBName:            relationship.ForeignDBNames[idx],
			AssociationDBName: dbName,
		})
	}

	s.Destination = JoinTableSource{ModelType: destination}
	s.Destination.ForeignKeys = []JoinTableForeignKey{}
	for idx, dbName := range relationship.AssociationForeignFieldNames {
		s.Destination.ForeignKeys = append(s.Destination.ForeignKeys, JoinTableForeignKey{
			DBName:            relationship.AssociationForeignDBNames[idx],
			AssociationDBName: dbName,
		})
	}
}

// Table return join table's table name
func (s JoinTableHandler) Table(db *DB) string {
	return DefaultTableNameHandler(db, s.TableName)
}

func (s JoinTableHandler) updateConditionMap(conditionMap map[string]interface{}, db *DB, joinTableSources []JoinTableSource, sources ...interface{}) {
	for _, source := range sources {
		scope := db.NewScope(source)
		modelType := scope.GetModelStruct().ModelType

		for _, joinTableSource := range joinTableSources {
			if joinTableSource.ModelType == modelType {
				for _, foreignKey := range joinTableSource.ForeignKeys {
					if field, ok := scope.FieldByName(foreignKey.AssociationDBName); ok {
						conditionMap[foreignKey.DBName] = field.Field.Interface()
					}
				}
				break
			}
		}
	}
}

// Add create relationship in join table for source and destination
func (s JoinTableHandler) Add(handler JoinTableHandlerInterface, db *DB, source interface{}, destination interface{}) error {
	var (
		scope        = db.NewScope("")
		conditionMap = map[string]interface{}{}
	)

	// Update condition map for source
	s.updateConditionMap(conditionMap, db, []JoinTableSource{s.Source}, source)

	// Update condition map for destination
	s.updateConditionMap(conditionMap, db, []JoinTableSource{s.Destination}, destination)

	var assignColumns, binVars, conditions []string
	var values []interface{}
	for key, value := range conditionMap {
		assignColumns = append(assignColumns, scope.Quote(key))
		binVars = append(binVars, `?`)
		conditions = append(conditions, fmt.Sprintf("%v = ?", scope.Quote(key)))
		values = append(values, value)
	}

	for _, value := range values {
		values = append(values, value)
	}

	quotedTable := scope.Quote(handler.Table(db))
	sql := fmt.Sprintf(
		"INSERT INTO %v (%v) SELECT %v %v WHERE NOT EXISTS (SELECT * FROM %v WHERE %v)",
		quotedTable,
		strings.Join(assignColumns, ","),
		strings.Join(binVars, ","),
		scope.Dialect().SelectFromDummyTable(),
		quotedTable,
		strings.Join(conditions, " AND "),
	)

	return db.Exec(sql, values...).Error
}
