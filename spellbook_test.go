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
		"create table entities (id integer not null primary key, label text)",
	}
	for _, sql := range sqls {
		_, err = db.Exec(sql)
		if err != nil {
			panic(err)
		}
	}
	return db
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
	db := getEmptyDB()

	m, err := NewManager(db)

	e, err := m.NewEntity()
	if err != nil {
		t.Error(err)
	}
	if e == nil {
		t.Error("nil Entity")
	}

	es, err := m.GetEntities()
	if err != nil {
		t.Error(err)
	}
	if !es.Next() || es.Entity() == nil {
		t.Fatal("no entity")
	}
	if es.Entity().id != e.id {
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

