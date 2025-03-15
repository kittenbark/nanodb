package nanodb

import (
	"iter"
	"sync"
	"time"
)

func New[T any]() *DB[T] {
	return &DB[T]{
		data:      make(map[string]T),
		lifetimes: make(map[string]time.Time),
		mutex:     &sync.RWMutex{},
	}
}

type DB[T any] struct {
	data      map[string]T
	lifetimes map[string]time.Time
	timeout   time.Duration
	mutex     *sync.RWMutex
}

func (db *DB[T]) Get(key string) T {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	return db.data[key]
}

func (db *DB[T]) GetOr(key string, otherwise T) T {
	if result, ok := db.TryGet(key); ok {
		return result
	}
	return otherwise
}

func (db *DB[T]) TryGet(key string) (T, bool) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	result, ok := db.data[key]
	return result, ok
}

func (db *DB[T]) Add(key string, value T) *DB[T] {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	db.data[key] = value
	db.lifetimes[key] = time.Now()
	db.scheduleDel(key)

	return db
}

func (db *DB[T]) Del(key string) *DB[T] {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	delete(db.data, key)
	delete(db.lifetimes, key)

	return db
}

func (db *DB[T]) Seq2() iter.Seq2[string, T] {
	return func(yield func(string, T) bool) {
		db.mutex.RLock()
		defer db.mutex.RUnlock()

		for key, value := range db.data {
			if !yield(key, value) {
				return
			}
		}
	}
}

func (db *DB[T]) KeysSnapshot() []string {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	keys := make([]string, 0, len(db.data))
	for key := range db.data {
		keys = append(keys, key)
	}
	return keys
}

func (db *DB[T]) Len() int {
	db.mutex.RLock()
	defer db.mutex.RUnlock()
	return len(db.data)
}

func (db *DB[T]) Timeout(timeout time.Duration) *DB[T] {
	db.timeout = timeout
	for key := range db.data {
		db.lifetimes[key] = time.Now()
		db.scheduleDel(key)
	}

	return db
}

func (db *DB[T]) scheduleDel(key string) {
	if db.timeout == 0 {
		return
	}

	time.AfterFunc(db.timeout, func() {
		db.mutex.Lock()
		defer db.mutex.Unlock()

		if time.Since(db.lifetimes[key]) >= db.timeout {
			delete(db.data, key)
			delete(db.lifetimes, key)
		}
	})
}
