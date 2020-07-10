package automigrate

// PrimaryFields return scope's primary fields
func (scope *Scope) PrimaryFields() (fields []*Field) {
	for _, field := range scope.Fields() {
		if field.IsPrimaryKey {
			fields = append(fields, field)
		}
	}
	return fields
}

// PrimaryField return scope's main primary field, if defined more that one primary fields, will return the one having column name `id` or the first one
func (scope *Scope) PrimaryField() *Field {
	if primaryFields := scope.GetModelStruct().PrimaryFields; len(primaryFields) > 0 {
		if len(primaryFields) > 1 {
			if field, ok := scope.FieldByName("id"); ok {
				return field
			}
		}
		return scope.PrimaryFields()[0]
	}
	return nil
}

// PrimaryKey get main primary field's db name
func (scope *Scope) PrimaryKey() string {
	if field := scope.PrimaryField(); field != nil {
		return field.DBName
	}
	return ""
}

// PrimaryKeyZero check main primary field's value is blank or not
func (scope *Scope) PrimaryKeyZero() bool {
	field := scope.PrimaryField()
	return field == nil || field.IsBlank
}

// PrimaryKeyValue get the primary key's value
func (scope *Scope) PrimaryKeyValue() interface{} {
	if field := scope.PrimaryField(); field != nil && field.Field.IsValid() {
		return field.Field.Interface()
	}
	return 0
}