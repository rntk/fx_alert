package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

type ValueType string

const (
	AboveCurrent ValueType = "<"
	BelowCurrent ValueType = ">"
)

type DB struct {
	l    sync.RWMutex
	path string
	db   map[string]map[string]map[string]ValueType
}

type Value struct {
	Key   string
	Value float64
	Type  ValueType
}

func (db *DB) Add(userID string, val Value) error {
	db.l.Lock()
	defer db.l.Unlock()
	if db.db == nil {
		db.db = map[string]map[string]map[string]ValueType{}
	}
	if db.db[userID] == nil {
		db.db[userID] = map[string]map[string]ValueType{}
	}
	if db.db[userID][val.Key] == nil {
		db.db[userID][val.Key] = map[string]ValueType{}
	}
	v := strconv.FormatFloat(val.Value, 'f', 5, 64)
	if _, exists := db.db[userID][val.Key][v]; !exists {
		db.db[userID][val.Key][v] = val.Type
	}

	return db.save()
}

func (db *DB) DeleteKey(userID string, key string) error {
	db.l.Lock()
	defer db.l.Unlock()
	db.deleteKey(userID, key)

	return db.save()
}

func (db *DB) deleteKey(userID string, key string) {
	if db.db == nil {
		return
	}
	if _, exists := db.db[userID]; !exists {
		return
	}
	delete(db.db[userID], key)
}

func (db *DB) DeleteValue(userID string, key string, value float64) error {
	db.l.Lock()
	defer db.l.Unlock()
	if db.db == nil {
		return nil
	}
	if db.db[userID] == nil {
		return nil
	}
	if db.db[userID][key] == nil {
		return nil
	}
	v := strconv.FormatFloat(value, 'f', 5, 64)
	delete(db.db[userID][key], v)
	if len(db.db[userID][key]) == 0 {
		db.deleteKey(userID, key)
	}

	return db.save()
}

func (db *DB) List(userID string) []Value {
	db.l.RLock()
	defer db.l.RUnlock()
	if db.db == nil {
		return nil
	}
	if db.db[userID] == nil {
		return nil
	}
	var lst []Value
	for k := range db.db[userID] {
		for rawV := range db.db[userID][k] {
			v, err := strconv.ParseFloat(rawV, 64)
			if err != nil {
				// TODO: panic or change db scheme?
				log.Printf("Can't parse float value from base: %q. %v", rawV, err)
				continue
			}
			lst = append(lst, Value{Key: k, Value: v, Type: db.db[userID][k][rawV]})
		}
	}

	return lst
}

func (db *DB) save() error {
	b, err := json.Marshal(db.db)
	if err != nil {
		return fmt.Errorf("Can`t marshal database: %w", err)
	}

	return ioutil.WriteFile(db.path, b, os.ModePerm)
}

func New(dbPath string, create bool) (*DB, error) {
	db := DB{path: dbPath, l: sync.RWMutex{}}
	b, err := ioutil.ReadFile(dbPath)
	if os.IsNotExist(err) && create {
		f, err := os.Create(dbPath)
		if err != nil {
			return nil, fmt.Errorf("Can't create database: %q. %w", dbPath, err)
		}
		defer f.Close()

		return &db, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Can't load database: %q.  %w", dbPath, err)
	}
	if len(b) == 0 {
		return &db, nil
	}
	if err := json.Unmarshal(b, &db.db); err != nil {
		return nil, fmt.Errorf("Can't unmarshal database: %q.  %w", dbPath, err)
	}

	return &db, nil
}

func ValueTypeFromString(txt string) (ValueType, error) {
	txt = strings.TrimSpace(txt)
	if txt == string(AboveCurrent) {
		return AboveCurrent, nil
	}
	if txt == string(BelowCurrent) {
		return BelowCurrent, nil
	}

	return "", errors.New("Unsupported type")
}
