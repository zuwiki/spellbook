type Manager struct {
  // todo stuff
}

func NewManager(db *sql.DB) (Manager, error) {}

type Entity struct {
	id uint64
	manager *Manager
}

type Component struct {
	name string
	manager *Manager
}

func (m *Manager) RegisterComponent(name string, table string, typ runtime.Type) error {}
func (m *Manager) RegisterLocalComponent(name string, typ runtime.Type) error {}

func (m *Manager) NewEntity() (Entity, error) {}
func (m *Manager) DeleteEntity(e Entity) error {}

func (e *Entity) Components() error {}
func (e *Entity) AddComponent(name string, c Component) error {}
func (e *Entity) GetComponent(name string) (Component, error) {}
func (e *Entity) RemoveComponent(name string) error {}

func (c *Component) Save() error {}

type Components struct {}

func (cs *Components) Close() error {}
func (cs *Components) Component() Component {}
func (cs *Components) Next() bool {}
func (cs *Components) Err() error {}

func (m *Manager) GetComponents(name string) (Components, error) {}
