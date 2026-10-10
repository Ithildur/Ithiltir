package mtlogin

type Store struct {
	mem *memState
}

func New() *Store {
	return &Store{mem: newMemory()}
}
