package automigrate

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"
)

func (scope *Scope) primaryCondition(value interface{}) string {
	return fmt.Sprintf("(%v.%v = %v)", scope.QuotedTableName(), scope.Quote(scope.PrimaryKey()), value)
}

func (scope *Scope) buildCondition(clause map[string]interface{}, include bool) (str string) {
	var (
		quotedTableName  = scope.QuotedTableName()
		quotedPrimaryKey = scope.Quote(scope.PrimaryKey())
		equalSQL         = "="
		inSQL            = "IN"
	)

	// If building not conditions
	if !include {
		equalSQL = "<>"
		inSQL = "NOT IN"
	}

	switch value := clause["query"].(type) {
	case sql.NullInt64:
		return fmt.Sprintf("(%v.%v %s %v)", quotedTableName, quotedPrimaryKey, equalSQL, value.Int64)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("(%v.%v %s %v)", quotedTableName, quotedPrimaryKey, equalSQL, value)
	case []int, []int8, []int16, []int32, []int64, []uint, []uint8, []uint16, []uint32, []uint64, []string, []interface{}:
		if !include && reflect.ValueOf(value).Len() == 0 {
			return
		}
		str = fmt.Sprintf("(%v.%v %s (?))", quotedTableName, quotedPrimaryKey, inSQL)
		clause["args"] = []interface{}{value}
	case string:
		if isNumberRegexp.MatchString(value) {
			return fmt.Sprintf("(%v.%v %s %v)", quotedTableName, quotedPrimaryKey, equalSQL, scope.AddToVars(value))
		}

		if value != "" {
			if !include {
				if comparisonRegexp.MatchString(value) {
					str = fmt.Sprintf("NOT (%v)", value)
				} else {
					str = fmt.Sprintf("(%v.%v NOT IN (?))", quotedTableName, scope.Quote(value))
				}
			} else {
				str = fmt.Sprintf("(%v)", value)
			}
		}
	case map[string]interface{}:
		var sqls []string
		for key, value := range value {
			if value != nil {
				sqls = append(sqls, fmt.Sprintf("(%v.%v %s %v)", quotedTableName, scope.Quote(key), equalSQL, scope.AddToVars(value)))
			} else {
				if !include {
					sqls = append(sqls, fmt.Sprintf("(%v.%v IS NOT NULL)", quotedTableName, scope.Quote(key)))
				} else {
					sqls = append(sqls, fmt.Sprintf("(%v.%v IS NULL)", quotedTableName, scope.Quote(key)))
				}
			}
		}
		return strings.Join(sqls, " AND ")
	case interface{}:
		var sqls []string
		newScope := scope.New(value)

		if len(newScope.Fields()) == 0 {
			scope.Err(fmt.Errorf("invalid query condition: %v", value))
			return
		}
		scopeQuotedTableName := newScope.QuotedTableName()
		for _, field := range newScope.Fields() {
			if !field.IsIgnored && !field.IsBlank {
				sqls = append(sqls, fmt.Sprintf("(%v.%v %s %v)", scopeQuotedTableName, scope.Quote(field.DBName), equalSQL, scope.AddToVars(field.Field.Interface())))
			}
		}
		return strings.Join(sqls, " AND ")
	default:
		scope.Err(fmt.Errorf("invalid query condition: %v", value))
		return
	}

	replacements := []string{}
	args := clause["args"].([]interface{})
	for _, arg := range args {
		var err error
		switch reflect.ValueOf(arg).Kind() {
		case reflect.Slice: // For where("id in (?)", []int64{1,2})
			if scanner, ok := interface{}(arg).(driver.Valuer); ok {
				arg, err = scanner.Value()
				replacements = append(replacements, scope.AddToVars(arg))
			} else if b, ok := arg.([]byte); ok {
				replacements = append(replacements, scope.AddToVars(b))
			} else if as, ok := arg.([][]interface{}); ok {
				var tempMarks []string
				for _, a := range as {
					var arrayMarks []string
					for _, v := range a {
						arrayMarks = append(arrayMarks, scope.AddToVars(v))
					}

					if len(arrayMarks) > 0 {
						tempMarks = append(tempMarks, fmt.Sprintf("(%v)", strings.Join(arrayMarks, ",")))
					}
				}

				if len(tempMarks) > 0 {
					replacements = append(replacements, strings.Join(tempMarks, ","))
				}
			} else if values := reflect.ValueOf(arg); values.Len() > 0 {
				var tempMarks []string
				for i := 0; i < values.Len(); i++ {
					tempMarks = append(tempMarks, scope.AddToVars(values.Index(i).Interface()))
				}
				replacements = append(replacements, strings.Join(tempMarks, ","))
			} else {
				replacements = append(replacements, scope.AddToVars(Expr("NULL")))
			}
		default:
			if valuer, ok := interface{}(arg).(driver.Valuer); ok {
				arg, err = valuer.Value()
			}

			replacements = append(replacements, scope.AddToVars(arg))
		}

		if err != nil {
			scope.Err(err)
		}
	}

	buff := bytes.NewBuffer([]byte{})
	i := 0
	for _, s := range str {
		if s == '?' && len(replacements) > i {
			buff.WriteString(replacements[i])
			i++
		} else {
			buff.WriteRune(s)
		}
	}

	str = buff.String()

	return
}

func (scope *Scope) buildSelectQuery(clause map[string]interface{}) (str string) {
	switch value := clause["query"].(type) {
	case string:
		str = value
	case []string:
		str = strings.Join(value, ", ")
	}

	args := clause["args"].([]interface{})
	replacements := []string{}
	for _, arg := range args {
		switch reflect.ValueOf(arg).Kind() {
		case reflect.Slice:
			values := reflect.ValueOf(arg)
			var tempMarks []string
			for i := 0; i < values.Len(); i++ {
				tempMarks = append(tempMarks, scope.AddToVars(values.Index(i).Interface()))
			}
			replacements = append(replacements, strings.Join(tempMarks, ","))
		default:
			if valuer, ok := interface{}(arg).(driver.Valuer); ok {
				arg, _ = valuer.Value()
			}
			replacements = append(replacements, scope.AddToVars(arg))
		}
	}

	buff := bytes.NewBuffer([]byte{})
	i := 0
	for pos, char := range str {
		if str[pos] == '?' {
			buff.WriteString(replacements[i])
			i++
		} else {
			buff.WriteRune(char)
		}
	}

	str = buff.String()

	return
}

// CombinedConditionSql return combined condition sql
func (scope *Scope) CombinedConditionSql() string {
	joinSQL := scope.joinsSQL()
	whereSQL := scope.whereSQL()
	if scope.Search.raw {
		whereSQL = strings.TrimSuffix(strings.TrimPrefix(whereSQL, "WHERE ("), ")")
	}
	return joinSQL + whereSQL + scope.groupSQL() +
		scope.havingSQL() + scope.orderSQL() + scope.limitAndOffsetSQL()
}
