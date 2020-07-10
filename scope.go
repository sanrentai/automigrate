package automigrate

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

type Scope struct {
	Search          *search
	Value           interface{}
	SQL             string
	SQLVars         []interface{}
	db              *DB
	instanceID      string
	primaryKeyField *Field
	skipLeft        bool
	fields          *[]*Field
	selectAttrs     *[]string
}

type tabler interface {
	TableName() string
}

type dbTabler interface {
	TableName(*DB) string
}

// NewDB create a new DB without search information
func (scope *Scope) NewDB() *DB {
	if scope.db != nil {
		db := scope.db.clone()
		db.search = nil
		db.Value = nil
		return db
	}
	return nil
}

// New create a new Scope without search information
func (scope *Scope) New(value interface{}) *Scope {
	return &Scope{db: scope.NewDB(), Search: &search{}, Value: value}
}

// Exec execute raw sql
func (s *DB) Exec(sql string, values ...interface{}) *DB {
	scope := s.NewScope(nil)
	generatedSQL := scope.buildCondition(map[string]interface{}{"query": sql, "args": values}, true)
	generatedSQL = strings.TrimSuffix(strings.TrimPrefix(generatedSQL, "("), ")")
	scope.Raw(generatedSQL)
	return scope.Exec().db
}

func (scope *Scope) TableName() string {
	if scope.Search != nil && len(scope.Search.tableName) > 0 {
		return scope.Search.tableName
	}

	if tabler, ok := scope.Value.(tabler); ok {
		return tabler.TableName()
	}

	if tabler, ok := scope.Value.(dbTabler); ok {
		return tabler.TableName(scope.db)
	}

	return scope.GetModelStruct().TableName(scope.db.Model(scope.Value))
}

// Dialect get dialect
func (scope *Scope) Dialect() Dialect {
	return scope.db.dialect
}

// Quote used to quote string to escape them for database
func (scope *Scope) Quote(str string) string {
	if strings.Contains(str, ".") {
		newStrs := []string{}
		for _, str := range strings.Split(str, ".") {
			newStrs = append(newStrs, scope.Dialect().Quote(str))
		}
		return strings.Join(newStrs, ".")
	}

	return scope.Dialect().Quote(str)
}

// QuotedTableName return quoted table name
func (scope *Scope) QuotedTableName() (name string) {
	if scope.Search != nil && len(scope.Search.tableName) > 0 {
		if strings.Contains(scope.Search.tableName, " ") {
			return scope.Search.tableName
		}
		return scope.Quote(scope.Search.tableName)
	}

	return scope.Quote(scope.TableName())
}

// InstanceID get InstanceID for scope
func (scope *Scope) InstanceID() string {
	if scope.instanceID == "" {
		scope.instanceID = fmt.Sprintf("%v%v", &scope, &scope.db)
	}
	return scope.instanceID
}

// InstanceGet get instance setting from current operation
func (scope *Scope) InstanceGet(name string) (interface{}, bool) {
	return scope.Get(name + scope.InstanceID())
}

// InstanceSet set instance setting for current operation, but not for operations in callbacks, like saving associations callback
func (scope *Scope) InstanceSet(name string, value interface{}) *Scope {
	return scope.Set(name+scope.InstanceID(), value)
}

// Err add error to Scope
func (scope *Scope) Err(err error) error {
	if err != nil {
		scope.db.AddError(err)
	}
	return err
}

// Raw set raw sql
func (scope *Scope) Raw(sql string) *Scope {
	scope.SQL = strings.Replace(sql, "$$$", "?", -1)
	return scope
}

// Exec perform generated SQL
func (scope *Scope) Exec() *Scope {
	if result, err := scope.db.db.Exec(scope.SQL, scope.SQLVars...); scope.Err(err) == nil {
		if count, err := result.RowsAffected(); scope.Err(err) == nil {
			scope.db.RowsAffected = count
		}
	}

	return scope
}

// AddToVars add value as sql's vars, used to prevent SQL injection
func (scope *Scope) AddToVars(value interface{}) string {
	_, skipBindVar := scope.InstanceGet("skip_bindvar")

	if expr, ok := value.(*SqlExpr); ok {
		exp := expr.expr
		for _, arg := range expr.args {
			if skipBindVar {
				scope.AddToVars(arg)
			} else {
				exp = strings.Replace(exp, "?", scope.AddToVars(arg), 1)
			}
		}
		return exp
	}

	scope.SQLVars = append(scope.SQLVars, value)

	if skipBindVar {
		return "?"
	}
	return scope.Dialect().BindVar(len(scope.SQLVars))
}

func (scope *Scope) autoMigrate() *Scope {
	tableName := scope.TableName()
	quotedTableName := scope.QuotedTableName()

	if !scope.Dialect().HasTable(tableName) {
		scope.createTable()
	} else {
		for _, field := range scope.GetModelStruct().StructFields {
			if !scope.Dialect().HasColumn(tableName, field.DBName) {
				if field.IsNormal {
					sqlTag := scope.Dialect().DataTypeOf(field)
					scope.Raw(fmt.Sprintf("ALTER TABLE %v ADD %v %v;", quotedTableName, scope.Quote(field.DBName), sqlTag)).Exec()
				}
			}
			scope.createJoinTable(field)
		}
		scope.autoIndex()
	}
	return scope
}

func (scope *Scope) autoIndex() *Scope {
	var indexes = map[string][]string{}
	var uniqueIndexes = map[string][]string{}

	for _, field := range scope.GetStructFields() {
		if name, ok := field.TagSettingsGet("INDEX"); ok {
			names := strings.Split(name, ",")

			for _, name := range names {
				if name == "INDEX" || name == "" {
					name = scope.Dialect().BuildKeyName("idx", scope.TableName(), field.DBName)
				}
				name, column := scope.Dialect().NormalizeIndexAndColumn(name, field.DBName)
				indexes[name] = append(indexes[name], column)
			}
		}

		if name, ok := field.TagSettingsGet("UNIQUE_INDEX"); ok {
			names := strings.Split(name, ",")

			for _, name := range names {
				if name == "UNIQUE_INDEX" || name == "" {
					name = scope.Dialect().BuildKeyName("uix", scope.TableName(), field.DBName)
				}
				name, column := scope.Dialect().NormalizeIndexAndColumn(name, field.DBName)
				uniqueIndexes[name] = append(uniqueIndexes[name], column)
			}
		}
	}

	for name, columns := range indexes {
		if db := scope.NewDB().Table(scope.TableName()).Model(scope.Value).AddIndex(name, columns...); db.Error != nil {
			scope.db.AddError(db.Error)
		}
	}

	for name, columns := range uniqueIndexes {
		if db := scope.NewDB().Table(scope.TableName()).Model(scope.Value).AddUniqueIndex(name, columns...); db.Error != nil {
			scope.db.AddError(db.Error)
		}
	}

	return scope
}

// Set set value by name
func (scope *Scope) Set(name string, value interface{}) *Scope {
	scope.db.InstantSet(name, value)
	return scope
}

// Get get setting by name
func (scope *Scope) Get(name string) (interface{}, bool) {
	return scope.db.Get(name)
}

// getTableOptions return the table options string or an empty string if the table options does not exist
func (scope *Scope) getTableOptions() string {
	tableOptions, ok := scope.Get("automigrate:table_options")
	if !ok {
		return ""
	}
	return " " + tableOptions.(string)
}

// IndirectValue return scope's reflect value's indirect value
func (scope *Scope) IndirectValue() reflect.Value {
	return indirect(reflect.ValueOf(scope.Value))
}

// Fields get value's fields
func (scope *Scope) Fields() []*Field {
	if scope.fields == nil {
		var (
			fields             []*Field
			indirectScopeValue = scope.IndirectValue()
			isStruct           = indirectScopeValue.Kind() == reflect.Struct
		)

		for _, structField := range scope.GetModelStruct().StructFields {
			if isStruct {
				fieldValue := indirectScopeValue
				for _, name := range structField.Names {
					if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
						fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
					}
					fieldValue = reflect.Indirect(fieldValue).FieldByName(name)
				}
				fields = append(fields, &Field{StructField: structField, Field: fieldValue, IsBlank: isBlank(fieldValue)})
			} else {
				fields = append(fields, &Field{StructField: structField, IsBlank: true})
			}
		}
		scope.fields = &fields
	}

	return *scope.fields
}

// FieldByName find `gorm.Field` with field name or db name
func (scope *Scope) FieldByName(name string) (field *Field, ok bool) {
	var (
		dbName           = ToColumnName(name)
		mostMatchedField *Field
	)

	for _, field := range scope.Fields() {
		if field.Name == name || field.DBName == name {
			return field, true
		}
		if field.DBName == dbName {
			mostMatchedField = field
		}
	}
	return mostMatchedField, mostMatchedField != nil
}

var (
	columnRegexp        = regexp.MustCompile("^[a-zA-Z\\d]+(\\.[a-zA-Z\\d]+)*$") // only match string like `name`, `users.name`
	isNumberRegexp      = regexp.MustCompile("^\\s*\\d+\\s*$")                   // match if string is number
	comparisonRegexp    = regexp.MustCompile("(?i) (=|<>|(>|<)(=?)|LIKE|IS|IN) ")
	countingQueryRegexp = regexp.MustCompile("(?i)^count(.+)$")
)

func (scope *Scope) quoteIfPossible(str string) string {
	if columnRegexp.MatchString(str) {
		return scope.Quote(str)
	}
	return str
}

func (scope *Scope) addIndex(unique bool, indexName string, column ...string) {
	if scope.Dialect().HasIndex(scope.TableName(), indexName) {
		return
	}

	var columns []string
	for _, name := range column {
		columns = append(columns, scope.quoteIfPossible(name))
	}

	sqlCreate := "CREATE INDEX"
	if unique {
		sqlCreate = "CREATE UNIQUE INDEX"
	}

	scope.Raw(fmt.Sprintf("%s %v ON %v(%v) %v", sqlCreate, indexName, scope.QuotedTableName(), strings.Join(columns, ", "), scope.whereSQL())).Exec()
}
