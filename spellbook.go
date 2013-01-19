package spellbook

import (
//	"reflect"
	"database/sql"
	"errors"
)

type Manager struct {
	db *sql.DB
}

func NewManager(db *sql.DB) (*Manager, error) {
	m := new(Manager)
	if db == nil {
		return nil, errors.New("need a database")
	}
	m.db = db
	return m, nil
}

type Entity struct {
	id int64
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
	r, err := m.db.Exec("insert into entities values (null)")
	if err != nil {
		return nil, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &Entity{id: id, manager: m}, nil
}
//func (m *Manager) DeleteEntity(e *Entity) error {}

type Entities struct {
	*sql.Rows
	manager *Manager
}

func (es *Entities) Entity() (*Entity, error)  {
	e := &Entity{manager: es.manager}
	err := es.Scan(&e.id)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (m *Manager) GetEntities() (*Entities, error) {
	rs, err := m.db.Query("select id from entities")
	if err != nil {
		return nil, err
	}
	return &Entities{rs, m}, nil
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
