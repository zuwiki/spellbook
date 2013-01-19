package spellbook

import (
//	"reflect"
	"database/sql"
)

type Manager struct {
}

func NewManager(db *sql.DB) (*Manager, error) {
	m := new(Manager)
	return m, nil
}

type Entity struct {
	id uint64
	manager *Manager
}

type Component struct {
	name string
	manager *Manager
}

//func (m *Manager) RegisterComponent(name string, table string, typ reflect.Type) error {}
//func (m *Manager) RegisterLocalComponent(name string, typ reflect.Type) error {}
func (m *Manager) GetComponentNames() []string {
	return []string{}
}

func (m *Manager) NewEntity() (*Entity, error) {
	return &Entity{}, nil
}
//func (m *Manager) DeleteEntity(e *Entity) error {}

type Entities struct {
	err error
}

func (es *Entities) Close() error {
	return nil
}
func (es *Entities) Entity() *Entity {
	return nil
}
func (es *Entities) Next() bool {
	return false
}
func (es *Entities) Err() error {
	return es.err
}

func (m *Manager) GetEntities() (Entities, error) {
	return Entities{}, nil
}

//func (e *Entity) Components() error {}
//func (e *Entity) AddComponent(name string, c Component) error {}
//func (e *Entity) GetComponent(name string) (Component, error) {}
//func (e *Entity) RemoveComponent(name string) error {}

//func (c *Component) Save() error {}

//type Components struct {}

//func (cs *Components) Close() error {}
//func (cs *Components) Component() Component {}
//func (cs *Components) Next() bool {}
//func (cs *Components) Err() error {}

//func (m *Manager) GetComponents(name string) (Components, error) {}
