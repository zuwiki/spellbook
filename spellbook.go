package spellbook

import (
	"reflect"
	"database/sql"
	// todo: use only fmt.Errorf?
	"errors"
	"fmt"
	"strings"
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
	entity int64
	name string
	isNew bool
	manager *Manager
	data interface{}
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
func (e *Entity) NewComponent(name string) (*Component, error) {
	ctype, ok := e.manager.componentTypes[name]
	if !ok {
		return nil, fmt.Errorf("Component %s doesn't exist", name)
	}
	r := e.manager.db.QueryRow("select entity_id from " + ctype.table + " where entity_id = ?", e.id)
	var id int64
	err := r.Scan(&id)
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("Entity already has %s component: %v", name, err)
	}
	c := Component{ entity: e.id, name: name, isNew: true, manager: e.manager, data: reflect.New(ctype.typ).Interface() }
	return &c, nil
}
func (e *Entity) GetComponent(name string) (*Component, error) {
	ctype, ok := e.manager.componentTypes[name]
	if !ok {
		return nil, fmt.Errorf("Entity doesn't have %s component", name)
	}
	rs, err := e.manager.db.Query("select * from " + ctype.table + " where entity_id = ?", e.id)
	defer rs.Close()
	if err != nil {
		return nil, err
	}
	if !rs.Next() {
		return nil, fmt.Errorf("No next row")
	}
	cols, err := rs.Columns()
	if err != nil {
		return nil, err
	}
	ifaces := make([]interface{}, len(cols))
	ifaceptrs := make([]interface{}, len(cols))
	for i := 0; i < len(ifaces); i++ {
		ifaceptrs[i] = &ifaces[i]
	}
	err = rs.Scan(ifaceptrs...)
	if err != nil {
		return nil, err
	}
	cv := reflect.New(ctype.typ).Elem()
	for i, field := range cols {
		if field == "entity_id" {
			continue
		}
		f := cv.FieldByName(field)
		if !f.IsValid() {
			return nil, fmt.Errorf("Field %s is invalid for %s", field, name)
		}
		iv := reflect.ValueOf(ifaces[i])
		switch iv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			f.SetInt(iv.Int())
		default:
			f.Set(iv)
		}
	}
	return &Component{ entity: e.id, name: name, isNew: false, manager: e.manager, data: cv.Addr().Interface() }, nil
}
//func (e *Entity) RemoveComponent(name string) error {}

func (c *Component) Save() error {
	ctype := c.manager.componentTypes[c.name]
	cv := reflect.ValueOf(c.data).Elem()
	if (ctype.typ != cv.Type()) {
		return fmt.Errorf("Incompatible types: expected %s, got %s", ctype.typ, cv.Type())
	}
	var query string
	if c.isNew {
		columnNames := make([]string, ctype.typ.NumField() + 1)
		for i := 0; i < len(columnNames) - 1; i++ {
			columnNames[i] = ctype.typ.Field(i).Name
		}
		columnNames[ctype.typ.NumField()] = "entity_id"
		questionMarks := make([]string, ctype.typ.NumField() + 1)
		for i := 0; i < len(questionMarks); i++ {
			questionMarks[i] = "?"
		}
		query = "insert into " + ctype.table + " (" + strings.Join(columnNames, ", ") +  ") values (" + strings.Join(questionMarks, ", ") + ")"
	} else {
		assignments := make([]string, ctype.typ.NumField())
		for i := 0; i < len(assignments); i++ {
			assignments[i] = ctype.typ.Field(i).Name + " = ?"
		}
		query = "update " + ctype.table + " set " + strings.Join(assignments, ", ") + " where entity_id = ?"
	}
	ifaces := make([]interface{}, ctype.typ.NumField() + 1)

	for i := 0; i < len(ifaces) - 1; i++ {
		ifaces[i] = cv.Field(i).Interface()
	}
	ifaces[ctype.typ.NumField()] = interface{}(c.entity)
	_, err := c.manager.db.Exec(query, ifaces...)
	if err != nil {
		return err
	}
	c.isNew = false
	return nil
}

//type Components struct {}

//func (cs *Components) Close() error {}
//func (cs *Components) Component() Component {}
//func (cs *Components) Next() bool {}
//func (cs *Components) Err() error {}

//func (m *Manager) GetComponents(name string) (Components, error) {}
