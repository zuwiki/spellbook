package spellbook

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var (
	ErrComponentNotRegistered = errors.New("No component registered with that name")
	ErrComponentAlreadyRegistered = errors.New("Component name already registered")
	ErrNoComponent = errors.New("Entity does not have that Component")
	ErrUnsatisfiedDependencies = errors.New("Entity lacks one or more dependencies of the desired component")
)

type componentType struct {
	table string
	typ reflect.Type
	local map[int64]interface{}
	dependencies []string
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

func (m *Manager) RegisterComponent(name string, table string, obj interface{}, deps []string) error {
	if _, ok := m.componentTypes[name]; ok {
		return ErrComponentAlreadyRegistered
	}
	if _, err := m.db.Exec("select 1 from " + table + " where 1 = 0"); err != nil {
		return err
	}
	m.componentTypes[name] = componentType{ table: table, typ: reflect.TypeOf(obj), dependencies: deps}
	return nil
}
func (m *Manager) RegisterLocalComponent(name string, obj interface{}, deps []string) error {
	if _, ok := m.componentTypes[name]; ok {
		return ErrComponentAlreadyRegistered
	}
	l := make(map[int64]interface{})
	m.componentTypes[name] = componentType{ typ: reflect.TypeOf(obj), local: l, dependencies: deps }
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

func (e *Entity) Delete() error {
	_, err := e.manager.db.Exec("delete from entities where id = ?", e.id)
	if err != nil {
		return err
	}
	return err
}

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

func (e *Entity) Components() ([]*Component, error) {
	cs := make([]*Component, 0)
	for name, _ := range e.manager.componentTypes {
		c, err := e.GetComponent(name)
		if err == nil {
			cs = append(cs, c)
		} else if err != ErrNoComponent {
			return nil, err
		}
	}
	return cs, nil
}

// Should only be called by Entity.NewComponent
func (e *Entity) newDbComponent(name string, ctype componentType) (*Component, error) {
	r := e.manager.db.QueryRow("select entity_id from " + ctype.table + " where entity_id = ?", e.id)
	var id int64
	err := r.Scan(&id)
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("Couldn't create component", name, err)
	}
	c := Component{ entity: e.id, name: name, isNew: true, manager: e.manager, data: reflect.New(ctype.typ).Interface() }
	return &c, nil
}

// Should only be called by Entity.NewComponent
func (e *Entity) newLocalComponent(name string, ctype componentType) (*Component, error) {
	_, ok := ctype.local[e.id]
	if ok {
		return nil, errors.New("Duplicate component")
	}
	c := Component{ entity: e.id, name: name, isNew: true, manager: e.manager, data: reflect.New(ctype.typ).Interface() }
	return &c, nil
}

func (e *Entity) NewComponent(name string) (*Component, error) {
	ctype, ok := e.manager.componentTypes[name]
	if !ok {
		return nil, ErrComponentNotRegistered
	}
	for _, dep := range ctype.dependencies {
		_, err := e.GetComponent(dep)
		if err != nil {
			return nil, ErrUnsatisfiedDependencies
		}
	}
	if ctype.local != nil {
		return e.newLocalComponent(name, ctype)
	}
	return e.newDbComponent(name, ctype)
}

func bindComponent(name string, rs *sql.Rows, ctype componentType, manager *Manager) (*Component, error) {
	var id int64
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
			id = ifaces[i].(int64)
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
	return &Component{ entity: id, name: name, isNew: false, manager: manager, data: cv.Addr().Interface() }, nil
}

// Should only be called by GetComponent
func (e *Entity) getLocalComponent(name string, ctype componentType) (*Component, error) {
	data, ok := ctype.local[e.id]
	if !ok {
		return nil, ErrNoComponent
	}
	return &Component{ entity: e.id, name: name, isNew: false, manager: e.manager, data: data }, nil
}

// Should only be called by GetComponent
func (e *Entity) getDbComponent(name string, ctype componentType) (*Component, error) {
	rs, err := e.manager.db.Query("select * from " + ctype.table + " where entity_id = ?", e.id)
	defer rs.Close()
	if err != nil {
		return nil, err
	}
	if !rs.Next() {
		return nil, ErrNoComponent
	}
	return bindComponent(name, rs, ctype, e.manager)
}

func (e *Entity) GetComponent(name string) (*Component, error) {
	ctype, ok := e.manager.componentTypes[name]
	if !ok {
		return nil, ErrComponentNotRegistered
	}
	if ctype.local != nil {
		return e.getLocalComponent(name, ctype)
	}
	return e.getDbComponent(name, ctype)
}

func (e *Entity) removeLocalComponent(name string, ctype componentType) error {
	delete(ctype.local, e.id)
	return nil
}

func (e *Entity) removeDbComponent(name string, ctype componentType) error {
	r, err := e.manager.db.Exec("delete from " + ctype.table + " where entity_id = ?", e.id)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrNoComponent
	}
	return nil
}

func (e *Entity) RemoveComponent(name string) error {
	ctype, ok := e.manager.componentTypes[name]
	if !ok {
		return ErrComponentNotRegistered
	}
	if ctype.local != nil {
		return e.removeLocalComponent(name, ctype)
	}
	return e.removeDbComponent(name, ctype)
}

func (c *Component) localSave(ctype componentType, cv reflect.Value) error {
	ctype.local[c.entity] = c.data
	c.isNew = false
	return nil
}

func (c *Component) dbSave(ctype componentType, cv reflect.Value) error {
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

func (c *Component) Save() error {
	ctype := c.manager.componentTypes[c.name]
	cv := reflect.ValueOf(c.data).Elem()
	if (ctype.typ != cv.Type()) {
		return fmt.Errorf("Incompatible types: expected %s, got %s", ctype.typ, cv.Type())
	}
	if ctype.local != nil {
		return c.localSave(ctype, cv)
	}
	return c.dbSave(ctype, cv)
}

type Components interface {
	Close() error
	Component() *Component
	Next() bool
	Err() error
}

type sliceComponents struct {
	slice []*Component
	index int
	closed bool
	err error
}

func (cs *sliceComponents) Close() error {
	cs.closed = true
	cs.slice = nil
	return cs.err
}

func (cs *sliceComponents) Component() *Component {
	if cs.closed {
		cs.err = errors.New("Iterator already closed")
		return nil
	}
	if cs.index < 0 {
		cs.err = errors.New("Next() not called on iterator")
		return nil
	}
	return cs.slice[cs.index]
}

func (cs *sliceComponents) Next() bool {
	if cs.closed {
		cs.err = errors.New("Iterator already closed")
		return false
	}
	cs.index += 1
	return cs.index < len(cs.slice)
}

func (cs *sliceComponents) Err() error {
	return cs.err
}

type dbComponents struct {
	rows *sql.Rows
	component *Component
	name string
	ctype componentType
	manager *Manager
	err error
}

func (cs *dbComponents) Close() error {
	return cs.rows.Close()
}
func (cs *dbComponents) Component() *Component {
	return cs.component
}
func (cs *dbComponents) Next() bool {
	if !cs.rows.Next() {
		return false
	}
	cs.component, cs.err = bindComponent(cs.name, cs.rows, cs.ctype, cs.manager)
	return cs.err == nil
}
func (cs *dbComponents) Err() error {
	if cs.err != nil {
		return cs.err
	}
	return cs.rows.Err()
}

func (m *Manager) GetComponents(name string) (Components, error) {
	return m.QueryComponent(name).Run()
}

type Query interface {
	Run() (Components, error)
	Where(string, interface{}, string)
}

type localQuery struct {
	name string
	ctype componentType
	manager *Manager
	wheres []func (reflect.Value) bool
}

type dbQuery struct {
	name string
	ctype componentType
	manager *Manager
	wheres []string
	args []interface{}
	err error
}

func (m *Manager) QueryComponent(name string) Query {
	ctype, ok := m.componentTypes[name]
	if !ok {
		return &dbQuery{ err: ErrComponentNotRegistered }
	}
	if ctype.local != nil {
		return &localQuery{ name: name, ctype: ctype, manager: m, wheres: make([]func (reflect.Value) bool, 0) }
	} else {
		return &dbQuery{ name: name, ctype: ctype, manager: m, wheres: make([]string, 0), args: make([]interface{}, 0) }
	}
	return nil
}

func (q *dbQuery) toString() string {
	s := "select * from " + q.ctype.table
	if len(q.wheres) > 0 {
		s += " where " + strings.Join(q.wheres, " and ")
	}
	return s
}

func (q *localQuery) Run() (Components, error) {
	cs := make([]*Component, 0)
	for id, data := range q.ctype.local {
		excluded := false
		c := Component{ entity: id, name: q.name, isNew: false, manager: q.manager, data: data }
		for _, pred := range q.wheres {
			cv := reflect.ValueOf(c.data)
			if !pred(cv) {
				excluded = true
				break
			}
		}
		if !excluded {
			cs = append(cs, &c)
		}
	}

	return &sliceComponents{ cs, -1, false, nil }, nil
}

func (q *localQuery) Where(field string, other interface{}, op string) {
	pred := func(val reflect.Value) bool {
		f := val.Elem().FieldByName(field)
		switch op {
		case "=":
			return f == reflect.ValueOf(other)
		case "!=":
			return f != reflect.ValueOf(other)
		case "<":
			switch f.Kind() {
			case reflect.String:
				return f.String() < other.(string)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return f.Int() < reflect.ValueOf(other).Int()
			case reflect.Float32, reflect.Float64:
				return f.Float() < reflect.ValueOf(other).Float()
			}
		case ">":
			switch f.Kind() {
			case reflect.String:
				return f.String() > other.(string)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return f.Int() > reflect.ValueOf(other).Int()
			case reflect.Float32, reflect.Float64:
				return f.Float() > reflect.ValueOf(other).Float()
			}
		case "<=":
			switch f.Kind() {
			case reflect.String:
				return f.String() <= other.(string)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return f.Int() <= reflect.ValueOf(other).Int()
			case reflect.Float32, reflect.Float64:
				return f.Float() <= reflect.ValueOf(other).Float()
			}
		case ">=":
			switch f.Kind() {
			case reflect.String:
				return f.String() >= other.(string)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return f.Int() >= reflect.ValueOf(other).Int()
			case reflect.Float32, reflect.Float64:
				return f.Float() >= reflect.ValueOf(other).Float()
			}
		}
		return false
	}
	q.wheres = append(q.wheres, pred)
}

func (q *dbQuery) Run() (Components, error) {
	if q.err != nil {
		return nil, q.err
	}
	rs, err := q.manager.db.Query(q.toString(), q.args...)
	if err != nil {
		return nil, err
	}
	c := new(dbComponents)
	c.rows = rs
	c.name = q.name
	c.ctype = q.ctype
	c.manager = q.manager
	return c, nil
}

func (q *dbQuery) Where(field string, val interface{}, op string) {
	// todo: check that field name exists
	s := fmt.Sprintf("%s %s ?", field, op)
	q.wheres = append(q.wheres, s)
	q.args = append(q.args, val)
}

func Eq(q Query, field string, val interface{}) {
	q.Where(field, val, "=")
}

func Gt(q Query, field string, val interface{}) {
	q.Where(field, val, ">")
}

func Gte(q Query, field string, val interface{}) {
	q.Where(field, val, ">=")
}

func Lt(q Query, field string, val interface{}) {
	q.Where(field, val, "<")
}

func Lte(q Query, field string, val interface{}) {
	q.Where(field, val, "<=")
}

func Neq(q Query, field string, val interface{}) {
	q.Where(field, val, "!=")
}

