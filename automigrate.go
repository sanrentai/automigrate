package automigrate

import (
	"sync"

	"github.com/gogf/gf/database/gdb"
)

type DB struct {
	sync.RWMutex
	Value        interface{}
	Error        error
	RowsAffected int64

	db        gdb.DB
	callbacks *Callback
	dialect   Dialect
	search    *search
	values    sync.Map
}

func NewDB(name string, db gdb.DB) *DB {
	return &DB{db: db, dialect: newDialect(name, db)}
}

func (s *DB) AutoMigrate(values ...interface{}) *DB {
	db := s.Unscoped()
	for _, value := range values {
		db = db.NewScope(value).autoMigrate().db
	}
	return db
}

// Model specify the model you would like to run db operations
//    // update all users's name to `hello`
//    db.Model(&User{}).Update("name", "hello")
//    // if user's primary key is non-blank, will use it as condition, then will only update the user's name to `hello`
//    db.Model(&user).Update("name", "hello")
func (s *DB) Model(value interface{}) *DB {
	c := s.clone()
	c.Value = value
	return c
}

// Table specify the table you would like to run db operations
func (s *DB) Table(name string) *DB {
	clone := s.clone()
	clone.search.Table(name)
	clone.Value = nil
	return clone
}

// Set set setting by name, which could be used in callbacks, will clone a new db, and update its setting
func (s *DB) Set(name string, value interface{}) *DB {
	return s.clone().InstantSet(name, value)
}

// InstantSet instant set setting, will affect current db
func (s *DB) InstantSet(name string, value interface{}) *DB {
	s.values.Store(name, value)
	return s
}

// Get get setting by name
func (s *DB) Get(name string) (value interface{}, ok bool) {
	value, ok = s.values.Load(name)
	return
}

func (s *DB) NewScope(value interface{}) *Scope {
	dbClone := s.clone()
	dbClone.Value = value
	scope := &Scope{db: dbClone, Value: value}
	if s.search != nil {
		scope.Search = s.search.clone()
	} else {
		scope.Search = &search{}
	}
	return scope
}

// Unscoped return all record including deleted record, refer Soft Delete https://jinzhu.github.io/gorm/crud.html#soft-delete
func (s *DB) Unscoped() *DB {
	return s.clone().search.unscoped().db
}

func (s *DB) clone() *DB {
	db := &DB{
		db:    s.db,
		Value: s.Value,
	}

	s.values.Range(func(k, v interface{}) bool {
		db.values.Store(k, v)
		return true
	})

	if s.search == nil {
		db.search = &search{limit: -1, offset: -1}
	} else {
		db.search = s.search.clone()
	}

	db.search.db = db
	return db
}

// AddError add error to the db
func (s *DB) AddError(err error) error {
	if err != nil {
		if err != ErrRecordNotFound {

			errors := Errors(s.GetErrors())
			errors = errors.Add(err)
			if len(errors) > 1 {
				err = errors
			}
		}

		s.Error = err
	}
	return err
}

// GetErrors get happened errors from the db
func (s *DB) GetErrors() []error {
	if errs, ok := s.Error.(Errors); ok {
		return errs
	} else if s.Error != nil {
		return []error{s.Error}
	}
	return []error{}
}

// AddIndex add index for columns with given name
func (s *DB) AddIndex(indexName string, columns ...string) *DB {
	scope := s.Unscoped().NewScope(s.Value)
	scope.addIndex(false, indexName, columns...)
	return scope.db
}

// AddUniqueIndex add unique index for columns with given name
func (s *DB) AddUniqueIndex(indexName string, columns ...string) *DB {
	scope := s.Unscoped().NewScope(s.Value)
	scope.addIndex(true, indexName, columns...)
	return scope.db
}
