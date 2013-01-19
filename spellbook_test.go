package spellbook

import (
	"testing"
	_ "github.com/mattn/go-sqlite3"
	"os"
	"database/sql"
)

const dbName = "./test.sqlite3"

func getEmptyDB() *sql.DB {
	os.Remove(dbName)
	db, err := sql.Open("sqlite3", dbName)
	// todo: further investigate cascading deletes
	sqls := []string{
		"create table entities (id integer not null primary key)",
		"create table xyz (entity_id integer not null primary key references entities(id) on delete cascade, X integer not null, Y integer not null, Z integer not null)",
		"create table nd (entity_id integer not null primary key references entities(id) on delete cascade, N text not null)",
	}
	for _, sql := range sqls {
		_, err = db.Exec(sql)
		if err != nil {
			panic(err)
		}
	}
	return db
}

func getEmptyManager() *Manager {
	db := getEmptyDB()
	m, _ := NewManager(db)
	return m
}

func TestEmptyManagerWithoutDb(t *testing.T) {
	_, err := NewManager(nil)
	if err == nil {
		t.Fatal("should fail without database")
	}
}

func TestEmptyManagerWithDb(t *testing.T) {
	db := getEmptyDB()

	m, err := NewManager(db)
	if err != nil {
		t.Fatal(err)
	}
	es, err := m.GetEntities()
	if err != nil {
		t.Error(err)
	}
	if es.Next() {
		t.Error("ghost entities")
	}
	es.Close()

	names := m.GetComponentNames()
	if len(names) != 0 {
		t.Error("ghost components")
	}
}

func TestMakingEntity(t *testing.T) {
	m := getEmptyManager()

	e, err := m.NewEntity()
	if err != nil {
		t.Fatal(err)
	}
	if e == nil {
		t.Error("nil Entity")
	}
	id := e.id
	e = nil

	es, err := m.GetEntities()
	if err != nil {
		t.Error(err)
	}
	if !es.Next() {
		t.Fatal("no entity", es.Err())
	}
	e, err = es.Entity()
	if err != nil || e == nil {
		t.Fatal("nil entity", err)
	}
	if id != e.id {
		t.Error("got wrong entity in a space of 1 entities")
	}
	if es.Next() {
		t.Error("ghost entities")
	}
	err = es.Close()
	if err != nil {
		t.Error(err)
	}
}

func TestDifferentEntityIds(t *testing.T) {
	m := getEmptyManager()

	e1, _ := m.NewEntity()
	e2, _ := m.NewEntity()
	if e1.id == e2.id {
		t.Error("got two entities with identical identities", e1, e2)
	}
}

type Xyz struct {
	X int
	Y int
	Z int
}

func TestRegisteringDbComponent(t *testing.T) {
	m := getEmptyManager()

	err := m.RegisterComponent("xyz!", "xyz", Xyz{})
	if err != nil {
		t.Fatal(err)
	}

	cns := m.GetComponentNames()
	if len(cns) != 1 {
		t.Fatal("Invalid component name count", len(cns))
	}
	if cns[0] != "xyz!" {
		t.Error("Expected xyz! for component name", cns[0])
	}
}

func TestRegisteringMissingDbComponent(t *testing.T) {
	m := getEmptyManager()

	err := m.RegisterComponent("xyz!", "notXyz!", Xyz{})
	if err == nil {
		t.Error("Registered a component with a missing table!")
	}

	cns := m.GetComponentNames()
	if len(cns) != 0 {
		t.Error("Component with missing table shows up in GetComponentNames!")
	}
}

func TestRegisteringComponentWithDuplicateName(t *testing.T) {
	m := getEmptyManager()

	err := m.RegisterComponent("xyz!", "xyz", Xyz{})
	if err != nil {
		t.Error(err)
	}
	err = m.RegisterComponent("xyz!", "xyz", Xyz{})
	if err == nil {
		t.Fatal("Registered DB component with duplicate name")
	}
	err = m.RegisterLocalComponent("xyz!", Xyz{})
	if err == nil {
		t.Fatal("Registered local component with duplicate name")
	}
}

func TestAddingUnregisteredComponent(t *testing.T) {
	m := getEmptyManager()

	e, _ := m.NewEntity()
	c, err := e.NewComponent("foo")
	if c != nil || err == nil {
		t.Error("Created component of nonexistent type")
	}
}

func TestCreatingAndSavingComponent(t *testing.T) {
	m := getEmptyManager()

	m.RegisterComponent("xyz!", "xyz", Xyz{})

	e, _ := m.NewEntity()

	c, err := e.NewComponent("xyz!")
	if c == nil || err != nil {
		t.Fatal("Failed to create component:", err)
	}
	xyz := c.data.(*Xyz)
	xyz.X = 1
	xyz.Y = 2
	xyz.Z = -1

	err = c.Save()
	if err != nil {
		t.Error(err)
	}

	c, err = e.GetComponent("xyz!")
	if c == nil || err != nil {
		t.Fatal("Failed to get component:", err)
	}
	xyz = c.data.(*Xyz)
	if xyz.X != 1 || xyz.Y != 2 || xyz.Z != -1 {
		t.Error("Retrieved wrong data", xyz)
	}
}

func TestUpdatingComponent(t *testing.T) {
	m := getEmptyManager()

	m.RegisterComponent("xyz!", "xyz", Xyz{})

	e, _ := m.NewEntity()

	c, _ := e.NewComponent("xyz!")
	xyz := c.data.(*Xyz)
	c.Save()

	c, _ = e.GetComponent("xyz!")
	xyz = c.data.(*Xyz)
	xyz.X = 1
	xyz.Y = 2
	xyz.Z = 3
	err := c.Save()
	if err != nil {
		t.Fatal("Failed to update component: ", err)
	}

	c, err = e.GetComponent("xyz!")
	if c == nil || err != nil {
		t.Fatal("Failed to get component after update:", err)
	}
	xyz = c.data.(*Xyz)
	if xyz.X != 1 || xyz.Y != 2 || xyz.Z != 3 {
		t.Error("Retrieved wrong data", xyz)
	}
}

func TestRemovingComponent(t *testing.T) {
	m := getEmptyManager()
	m.RegisterComponent("xyz!", "xyz", Xyz{})

	e, _ := m.NewEntity()
	err := e.RemoveComponent("foo")
	if err != ErrComponentNotRegistered {
		t.Error("Removed an unregistered component")
	}
	err = e.RemoveComponent("xyz!")
	if err != ErrNoComponent {
		t.Error("Removed a registered but nonexistent component")
	}

	c, _ := e.NewComponent("xyz!")
	c.Save()

	err = e.RemoveComponent("xyz!")
	if err != nil {
		t.Error("Failed to remove a registered, existent component")
	}
}

func TestDeletingEntity(t *testing.T) {
	m := getEmptyManager()

	m.RegisterComponent("xyz!", "xyz", Xyz{})

	e, _ := m.NewEntity()
	err := e.Delete()
	if err != nil {
		t.Error("Failed to delete empty entity")
	}

	e, _ = m.NewEntity()

	c, _ := e.NewComponent("xyz!")
	c.Save()

	err = e.Delete()

	es, err := m.GetEntities()
	if err != nil {
		t.Error(err)
	}
	if es.Next() {
		t.Error("Failed to actually delete empty entity")
	}
	es.Close()

	// todo: test Components list to make sure component was deleted
}

type Nd struct {
	N string
}

func TestComponentList(t *testing.T) {
	m := getEmptyManager()

	m.RegisterComponent("xyz!", "xyz", Xyz{})
	m.RegisterComponent("N?", "nd", Nd{})

	e1, _ := m.NewEntity()
	e2, _ := m.NewEntity()

	c, _ := e1.NewComponent("xyz!")
	c.Save()

	c, _ = e1.NewComponent("N?")
	n := c.data.(*Nd)
	n.N = "e1"
	c.Save()

	c, _ = e2.NewComponent("N?")
	n = c.data.(*Nd)
	n.N = "e2"
	c.Save()

	ns, err := m.GetComponents("N?")
	if err != nil {
		t.Fatal("Error getting iterator:", err)
	}
	if !ns.Next() || ns.Err() != nil || ns.Component() == nil {
		t.Fatal("0 values in components", ns.Err())
	}
	n1Seen := false
	n2Seen := false
	n = ns.Component().data.(*Nd)
	if n.N == "e1" {
		n1Seen = true
	} else if n.N == "e2" {
		n2Seen = true
	}
	if !ns.Next() || ns.Err() != nil || ns.Component() == nil {
		t.Fatal("only 1 values in components", ns.Err())
	}
	n = ns.Component().data.(*Nd)
	if n.N == "e1" {
		n1Seen = true
	} else if n.N == "e2" {
		n2Seen = true
	}
	if !n1Seen || !n2Seen {
		t.Error("Either n1 or n2 missing")
	}
	if ns.Next() || ns.Err() != nil {
		t.Error("Extra component in iterator", ns.Err())
	}
}
