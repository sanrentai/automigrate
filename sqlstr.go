package automigrate

import (
	"fmt"
	"strings"
)

func (scope *Scope) whereSQL() (sql string) {
	var (
		quotedTableName                                = scope.QuotedTableName()
		deletedAtField, hasDeletedAtField              = scope.FieldByName("DeletedAt")
		primaryConditions, andConditions, orConditions []string
	)

	if !scope.Search.Unscoped && hasDeletedAtField {
		sql := fmt.Sprintf("%v.%v IS NULL", quotedTableName, scope.Quote(deletedAtField.DBName))
		primaryConditions = append(primaryConditions, sql)
	}

	if !scope.PrimaryKeyZero() {
		for _, field := range scope.PrimaryFields() {
			sql := fmt.Sprintf("%v.%v = %v", quotedTableName, scope.Quote(field.DBName), scope.AddToVars(field.Field.Interface()))
			primaryConditions = append(primaryConditions, sql)
		}
	}

	for _, clause := range scope.Search.whereConditions {
		if sql := scope.buildCondition(clause, true); sql != "" {
			andConditions = append(andConditions, sql)
		}
	}

	for _, clause := range scope.Search.orConditions {
		if sql := scope.buildCondition(clause, true); sql != "" {
			orConditions = append(orConditions, sql)
		}
	}

	for _, clause := range scope.Search.notConditions {
		if sql := scope.buildCondition(clause, false); sql != "" {
			andConditions = append(andConditions, sql)
		}
	}

	orSQL := strings.Join(orConditions, " OR ")
	combinedSQL := strings.Join(andConditions, " AND ")
	if len(combinedSQL) > 0 {
		if len(orSQL) > 0 {
			combinedSQL = combinedSQL + " OR " + orSQL
		}
	} else {
		combinedSQL = orSQL
	}

	if len(primaryConditions) > 0 {
		sql = "WHERE " + strings.Join(primaryConditions, " AND ")
		if len(combinedSQL) > 0 {
			sql = sql + " AND (" + combinedSQL + ")"
		}
	} else if len(combinedSQL) > 0 {
		sql = "WHERE " + combinedSQL
	}
	return
}

func (scope *Scope) selectSQL() string {
	if len(scope.Search.selects) == 0 {
		if len(scope.Search.joinConditions) > 0 {
			return fmt.Sprintf("%v.*", scope.QuotedTableName())
		}
		return "*"
	}
	return scope.buildSelectQuery(scope.Search.selects)
}

func (scope *Scope) orderSQL() string {
	if len(scope.Search.orders) == 0 || scope.Search.ignoreOrderQuery {
		return ""
	}

	var orders []string
	for _, order := range scope.Search.orders {
		if str, ok := order.(string); ok {
			orders = append(orders, scope.quoteIfPossible(str))
		} else if expr, ok := order.(*SqlExpr); ok {
			exp := expr.expr
			for _, arg := range expr.args {
				exp = strings.Replace(exp, "?", scope.AddToVars(arg), 1)
			}
			orders = append(orders, exp)
		}
	}
	return " ORDER BY " + strings.Join(orders, ",")
}

func (scope *Scope) limitAndOffsetSQL() string {
	sql, err := scope.Dialect().LimitAndOffsetSQL(scope.Search.limit, scope.Search.offset)
	scope.Err(err)
	return sql
}

func (scope *Scope) groupSQL() string {
	if len(scope.Search.group) == 0 {
		return ""
	}
	return " GROUP BY " + scope.Search.group
}

func (scope *Scope) havingSQL() string {
	if len(scope.Search.havingConditions) == 0 {
		return ""
	}

	var andConditions []string
	for _, clause := range scope.Search.havingConditions {
		if sql := scope.buildCondition(clause, true); sql != "" {
			andConditions = append(andConditions, sql)
		}
	}

	combinedSQL := strings.Join(andConditions, " AND ")
	if len(combinedSQL) == 0 {
		return ""
	}

	return " HAVING " + combinedSQL
}

func (scope *Scope) joinsSQL() string {
	var joinConditions []string
	for _, clause := range scope.Search.joinConditions {
		if sql := scope.buildCondition(clause, true); sql != "" {
			joinConditions = append(joinConditions, strings.TrimSuffix(strings.TrimPrefix(sql, "("), ")"))
		}
	}

	return strings.Join(joinConditions, " ") + " "
}

func (scope *Scope) prepareQuerySQL() {
	if scope.Search.raw {
		scope.Raw(scope.CombinedConditionSql())
	} else {
		scope.Raw(fmt.Sprintf("SELECT %v FROM %v %v", scope.selectSQL(), scope.QuotedTableName(), scope.CombinedConditionSql()))
	}
	return
}
