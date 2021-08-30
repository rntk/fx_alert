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
	db   map[int64]map[string]map[string]ValueType
}

type Value struct {
	Key       string
	Value     float64
	Type      ValueType
	Precision uint8
}

func (v Value) IsAlert(currentV float64) bool {
	if currentV >= v.Value {
		if v.Type == BelowCurrent {
			return true
		}
	}

	if currentV <= v.Value {
		if v.Type == AboveCurrent {
			return true
		}
	}

	return false
}

func (v Value) String() string {
	return fmt.Sprintf("%s %s %s", v.Key, v.Type, v.StringValue())
}

func (v Value) StringValue() string {
	return strconv.FormatFloat(v.Value, 'f', int(v.Precision), 64)
}

func (db *DB) Add(ID int64, values []Value) error {
	db.l.Lock()
	defer db.l.Unlock()
	if db.db == nil {
		db.db = map[int64]map[string]map[string]ValueType{}
	}
	if db.db[ID] == nil {
		db.db[ID] = map[string]map[string]ValueType{}
	}
	for _, val := range values {
		key := strings.ToUpper(val.Key)
		if db.db[ID][key] == nil {
			db.db[ID][key] = map[string]ValueType{}
		}
		v := val.StringValue()
		if _, exists := db.db[ID][key][v]; !exists {
			db.db[ID][key][v] = val.Type
		}
	}

	return db.save()
}

func (db *DB) DeleteKey(ID int64, key string) error {
	db.l.Lock()
	defer db.l.Unlock()
	db.deleteKey(ID, key)

	return db.save()
}

func (db *DB) deleteKey(ID int64, key string) {
	if db.db == nil {
		return
	}
	if _, exists := db.db[ID]; !exists {
		return
	}
	delete(db.db[ID], key)
}

func (db *DB) DeleteValue(ID int64, val Value) error {
	db.l.Lock()
	defer db.l.Unlock()
	if db.db == nil {
		return nil
	}
	if db.db[ID] == nil {
		return nil
	}
	if db.db[ID][val.Key] == nil {
		return nil
	}
	v := val.StringValue()
	delete(db.db[ID][val.Key], v)
	if len(db.db[ID][val.Key]) == 0 {
		db.deleteKey(ID, val.Key)
	}

	return db.save()
}

func (db *DB) List(ID int64) []Value {
	db.l.RLock()
	defer db.l.RUnlock()
	if db.db == nil {
		return nil
	}
	if db.db[ID] == nil {
		return nil
	}
	var lst []Value
	for k := range db.db[ID] {
		for rawV := range db.db[ID][k] {
			parts := strings.Split(rawV, ".")
			prec := 5
			if len(parts) > 1 {
				prec = len(parts[1])
			}
			v, err := strconv.ParseFloat(rawV, 64)
			if err != nil {
				// TODO: panic or change db scheme?
				log.Printf("Can't parse float value from base: %q. %v", rawV, err)
				continue
			}
			lst = append(lst, Value{Key: k, Value: v, Type: db.db[ID][k][rawV], Precision: uint8(prec)})
		}
	}

	return lst
}

func (db *DB) Users() []int64 {
	db.l.RLock()
	defer db.l.RUnlock()
	if db.db == nil {
		return nil
	}
	var lst []int64
	for ID := range db.db {
		lst = append(lst, ID)
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
