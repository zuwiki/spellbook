package spellbook

import (
	"reflect"
	"database/sql"
	"errors"
)

type componentType struct {
	table string
	typ reflect.Type
	local bool
}

type Manager struct {
	db *sql.DB
	componentTypes map[string] componentType
}

func NewManager(db *sql.DB) (*Manager, error) {
	m := new(Manager)
	if db == nil {
		return nil, errors.New("need a database")
	}
	m.db = db
	m.componentTypes = make(map[string] componentType)
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

func (m *Manager) RegisterComponent(name string, table string, obj interface{}) error {
	if _, ok := m.componentTypes[name]; ok {
		return errors.New("Component name already registered")
	}
	if _, err := m.db.Exec("select 1 from " + table + " where 1 = 0"); err != nil {
		return err
	}
	m.componentTypes[name] = componentType{ table: table, typ: reflect.TypeOf(obj) }
	return nil
}
func (m *Manager) RegisterLocalComponent(name string, obj interface{}) error {
	if _, ok := m.componentTypes[name]; ok {
		return errors.New("Component name already registered")
	}
	m.componentTypes[name] = componentType{ typ: reflect.TypeOf(obj), local: true }
	return nil
}
func (m *Manager) GetComponentNames() []string {
	names := []string{}
	for name, _ := range m.componentTypes {
		names = append(names, name)
	}
	return names
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
