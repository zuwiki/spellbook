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
		return ErrComponentAlreadyRegistered
	}
	if _, err := m.db.Exec("select 1 from " + table + " where 1 = 0"); err != nil {
		return err
	}
	m.componentTypes[name] = componentType{ table: table, typ: reflect.TypeOf(obj) }
	return nil
}
func (m *Manager) RegisterLocalComponent(name string, obj interface{}) error {
	if _, ok := m.componentTypes[name]; ok {
		return ErrComponentAlreadyRegistered
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

func (e *Entity) NewComponent(name string) (*Component, error) {
	ctype, ok := e.manager.componentTypes[name]
	if !ok {
		return nil, ErrComponentNotRegistered
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

func (e *Entity) GetComponent(name string) (*Component, error) {
	ctype, ok := e.manager.componentTypes[name]
	if !ok {
		return nil, ErrComponentNotRegistered
	}
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

func (e *Entity) RemoveComponent(name string) error {
	ctype, ok := e.manager.componentTypes[name]
	if !ok {
		return ErrComponentNotRegistered
	}
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

type Components struct {
	rows *sql.Rows
	component *Component
	name string
	ctype componentType
	manager *Manager
	err error
}

func (cs *Components) Close() error {
	return cs.rows.Close()
}
func (cs *Components) Component() *Component {
	return cs.component
}
func (cs *Components) Next() bool {
	if !cs.rows.Next() {
		return false
	}
	cs.component, cs.err = bindComponent(cs.name, cs.rows, cs.ctype, cs.manager)
	return cs.err == nil
}
func (cs *Components) Err() error {
	if cs.err != nil {
		return cs.err
	}
	return cs.rows.Err()
}

func (m *Manager) GetComponents(name string) (*Components, error) {
	return m.QueryComponent(name).Run()
}

type Query struct {
	name string
	ctype componentType
	manager *Manager
	wheres []string
	args []interface{}
	err error
}

func (m *Manager) QueryComponent(name string) *Query {
	ctype, ok := m.componentTypes[name]
	if !ok {
		return &Query{ err: ErrComponentNotRegistered }
	}
	return &Query{ name: name, ctype: ctype, manager: m, wheres: make([]string, 0), args: make([]interface{}, 0) }
}

func (q *Query) toString() string {
	s := "select * from " + q.ctype.table
	if len(q.wheres) > 0 {
		s += " where " + strings.Join(q.wheres, " and ")
	}
	return s
}

func (q *Query) Run() (*Components, error) {
	if q.err != nil {
		return nil, q.err
	}
	rs, err := q.manager.db.Query(q.toString(), q.args...)
	if err != nil {
		return nil, err
	}
	c := new(Components)
	c.rows = rs
	c.name = q.name
	c.ctype = q.ctype
	c.manager = q.manager
	return c, nil
}

func (q *Query) Where(field string, val interface{}, op string) {
	// todo: check that field name exists
	s := fmt.Sprintf("%s %s ?", field, op)
	q.wheres = append(q.wheres, s)
	q.args = append(q.args, val)
}

func (q *Query) Eq(field string, val interface{}) {
	q.Where(field, val, "=")
}

func (q *Query) Gt(field string, val interface{}) {
	q.Where(field, val, ">")
}

func (q *Query) Gte(field string, val interface{}) {
	q.Where(field, val, ">=")
}

func (q *Query) Lt(field string, val interface{}) {
	q.Where(field, val, "<")
}

func (q *Query) Lte(field string, val interface{}) {
	q.Where(field, val, "<=")
}

func (q *Query) Neq(field string, val interface{}) {
	q.Where(field, val, "!=")
}

