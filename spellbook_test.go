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
	sqls := []string{
		"create table entities (id integer not null primary key)",
		"create table xyz (entity_id integer not null primary key, X integer not null, Y integer not null, Z integer not null)",
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


