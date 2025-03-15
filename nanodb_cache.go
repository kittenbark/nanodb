package nanodb

import (
	"encoding/json"
	"io"
	"iter"
	"log/slog"
	"os"
	"sync"
	"time"
)

type Encoder interface {
	Encode(any) error
}
type Decoder interface {
	Decode(any) error
}

type NewEncoder[T Encoder] func(w io.Writer) T
type NewDecoder[T Decoder] func(r io.Reader) T

func Fromf[T any, EncoderT Encoder, DecoderT Decoder](
	filename string,
	encoder NewEncoder[EncoderT],
	decoder NewDecoder[DecoderT],
) (*DBCache[T, EncoderT, DecoderT], error) {
	db := &DBCache[T, EncoderT, DecoderT]{
		cache:      filename,
		data:       make(map[string]T),
		lifetimes:  make(map[string]time.Time),
		mutex:      &sync.Mutex{},
		newEncoder: encoder,
		newDecoder: decoder,
	}
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		if err := db.save(); err != nil {
			return nil, err
		}
	}
	return db, db.load()
}

func From[T any](filename string) (*DBCache[T, *json.Encoder, *json.Decoder], error) {
	return Fromf[T](filename, json.NewEncoder, json.NewDecoder)
}

type DBCache[T any, EncoderT Encoder, DecoderT Decoder] struct {
	cache      string
	data       map[string]T
	lifetimes  map[string]time.Time
	timeout    time.Duration
	mutex      *sync.Mutex
	lastSync   time.Time
	newEncoder NewEncoder[EncoderT]
	newDecoder NewDecoder[DecoderT]
}

func (db *DBCache[T, EncoderT, DecoderT]) Get(key string) (result T, err error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if err = db.load(); err != nil {
		return
	}
	return db.data[key], nil
}

func (db *DBCache[T, EncoderT, DecoderT]) TryGet(key string) (result T, ok bool, err error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if err = db.load(); err != nil {
		return
	}
	result, ok = db.data[key]
	return
}

func (db *DBCache[T, EncoderT, DecoderT]) Add(key string, value T) error {
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

func (db *DBCache[T, EncoderT, DecoderT]) Del(key string) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if db.timeout != 0 && time.Since(db.lifetimes[key]) >= db.timeout {
		delete(db.data, key)
		delete(db.lifetimes, key)

		if err := db.save(); err != nil {
			slog.Error("nanodb-cache", "del", key, "err", err)
		}
	}

	return db.save()
}

func (db *DBCache[T, EncoderT, DecoderT]) Seq2() iter.Seq2[string, T] {
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

func (db *DBCache[T, EncoderT, DecoderT]) Len() (int, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if err := db.load(); err != nil {
		return 0, err
	}

	return len(db.data), nil
}

func (db *DBCache[T, EncoderT, DecoderT]) KeysSnapshot() ([]string, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if err := db.load(); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(db.data))
	for key := range db.data {
		keys = append(keys, key)
	}
	return keys, nil
}

func (db *DBCache[T, EncoderT, DecoderT]) Timeout(timeout time.Duration) *DBCache[T, EncoderT, DecoderT] {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	db.timeout = timeout
	for key := range db.data {
		db.lifetimes[key] = time.Now()
		db.scheduleDel(key)
	}

	return db
}

func (db *DBCache[T, EncoderT, DecoderT]) scheduleDel(key string) {
	if db.timeout == 0 {
		return
	}

	time.AfterFunc(db.timeout, func() {
		db.Del(key)
	})
}

func (db *DBCache[T, EncoderT, DecoderT]) load() error {
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

	return db.newDecoder(cache).Decode(&db.data)
}

func (db *DBCache[T, EncoderT, DecoderT]) save() error {
	cache, err := os.OpenFile(db.cache, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	defer func() {
		if e := cache.Close(); err == nil && e != nil {
			err = e
		}
	}()

	return db.newEncoder(cache).Encode(db.data)
}
