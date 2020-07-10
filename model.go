package automigrate

import "time"

type Model struct {
	ID        uint `automigrate:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `sql:"index"`
}