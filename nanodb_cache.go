package nanodb

import (
	"encoding/json"
	"iter"
	"log/slog"
	"os"
	"sync"
	"time"
)

func From[T any](filename string) (*DBCache[T], error) {
	db := &DBCache[T]{
		cache:     filename,
		data:      make(map[string]T),
		lifetimes: make(map[string]time.Time),
		mutex:     &sync.Mutex{},
	}
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		if err := db.save(); err != nil {
			return nil, err
		}
	}
	return db, db.load()
}

type DBCache[T any] struct {
	cache     string
	data      map[string]T
	lifetimes map[string]time.Time
	timeout   time.Duration
	mutex     *sync.Mutex
	lastSync  time.Time
}

func (db *DBCache[T]) Get(key string) (result T, err error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if err = db.load(); err != nil {
		return
	}
	return db.data[key], nil
}

func (db *DBCache[T]) TryGet(key string) (result T, ok bool, err error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if err = db.load(); err != nil {
		return
	}
	result, ok = db.data[key]
	return
}

func (db *DBCache[T]) Add(key string, value T) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if err := db.load(); err != nil {
		return err
	}

	db.data[key] = value
	db.lifetimes[key] = time.Now()
	db.scheduleDel(key)

	return db.save()
}

func (db *DBCache[T]) Del(key string) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if time.Since(db.lifetimes[key]) >= db.timeout {
		delete(db.data, key)
		delete(db.lifetimes, key)

		if err := db.save(); err != nil {
			slog.Error("nanodb-cache", "del", key, "err", err)
		}
	}

	return db.save()
}

func (db *DBCache[T]) Seq2() iter.Seq2[string, T] {
	return func(yield func(string, T) bool) {
		db.mutex.Lock()
		defer db.mutex.Unlock()

		_ = db.load()
		for key, value := range db.data {
			if !yield(key, value) {
				return
			}
		}
	}
}

func (db *DBCache[T]) Len() (int, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if err := db.load(); err != nil {
		return 0, err
	}

	return len(db.data), nil
}

func (db *DBCache[T]) Timeout(timeout time.Duration) *DBCache[T] {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	db.timeout = timeout
	for key := range db.data {
		db.lifetimes[key] = time.Now()
		db.scheduleDel(key)
	}

	return db
}

func (db *DBCache[T]) scheduleDel(key string) {
	if db.timeout == 0 {
		return
	}

	time.AfterFunc(db.timeout, func() {
		db.Del(key)
	})
}

func (db *DBCache[T]) load() error {
	stat, err := os.Stat(db.cache)
	if err != nil {
		return err
	}
	if stat.ModTime().Before(db.lastSync) {
		return nil
	}

	db.lastSync = stat.ModTime()
	cache, err := os.Open(db.cache)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	defer func() {
		if e := cache.Close(); err == nil && e != nil {
			err = e
		}
	}()

	return json.NewDecoder(cache).Decode(&db.data)
}

func (db *DBCache[T]) save() error {
	cache, err := os.OpenFile(db.cache, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	defer func() {
		if e := cache.Close(); err == nil && e != nil {
			err = e
		}
	}()

	return json.NewEncoder(cache).Encode(db.data)
}
