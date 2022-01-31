package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
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

var ErrUserNotFound = errors.New("User not found")

type DB struct {
	l    sync.RWMutex
	path string
	db   map[int64]UserData
}

type UserData struct {
	Settings UserSettings
	Levels   map[string][]Value
}

type Level struct {
	Type  ValueType
	Delta int
}

type UserSettings struct {
	Delta float64
}

type Value struct {
	Key       string
	Value     float64
	Type      ValueType
	Precision uint8
	Delta     uint64
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

func (db *DB) initUser(ID int64) {
	if db.db == nil {
		db.db = map[int64]UserData{}
	}
	if _, exists := db.db[ID]; !exists {
		db.db[ID] = UserData{
			Levels: map[string][]Value{},
		}
	}
}

func (db *DB) Add(ID int64, values []Value) error {
	db.l.Lock()
	defer db.l.Unlock()
	db.initUser(ID)
	for _, val := range values {
		key := strings.ToUpper(val.Key)
		if db.db[ID].Levels[key] == nil {
			db.db[ID].Levels[key] = []Value{}
		}
		exists := false
		for _, dbV := range db.db[ID].Levels[key] {
			if val.Value == dbV.Value {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		db.db[ID].Levels[key] = append(db.db[ID].Levels[key], val)
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
	delete(db.db[ID].Levels, key)
}

func (db *DB) deleteValue(ID int64, key string, pos int) {
	if db.db == nil {
		return
	}
	if db.db[ID].Levels == nil {
		return
	}
	key = strings.ToUpper(key)
	if len(db.db[ID].Levels[key]) == 0 {
		return
	}

	if pos >= 0 {
		ln := len(db.db[ID].Levels[key])
		if ln > 1 {
			db.db[ID].Levels[key][pos] = db.db[ID].Levels[key][ln-1]
			db.db[ID].Levels[key] = db.db[ID].Levels[key][:ln-1]
		} else {
			db.db[ID].Levels[key] = nil
		}
	}
}

func (db *DB) DeleteValue(ID int64, val Value) error {
	db.l.Lock()
	defer db.l.Unlock()
	for i, v := range db.db[ID].Levels[val.Key] {
		if v.Value == val.Value {
			db.deleteValue(ID, val.Key, i)
			break
		}
	}
	if val.Delta > 0 {
		for i, v := range db.db[ID].Levels[val.Key] {
			if v.Delta == val.Delta {
				db.deleteValue(ID, val.Key, i)
				break
			}
		}
	}
	if len(db.db[ID].Levels[val.Key]) == 0 {
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
	if db.db[ID].Levels == nil {
		return nil
	}
	var lst []Value
	for k := range db.db[ID].Levels {
		lst = append(lst, db.db[ID].Levels[k]...)
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

func (db *DB) GetSettings(ID int64) (*UserSettings, error) {
	db.l.RLock()
	defer db.l.RUnlock()
	if _, exists := db.db[ID]; !exists {
		return nil, ErrUserNotFound
	}
	s := db.db[ID].Settings

	return &s, nil
}

func (db *DB) SetSettings(ID int64, settings UserSettings) error {
	db.l.Lock()
	defer db.l.Unlock()
	db.initUser(ID)
	u := db.db[ID]
	u.Settings = settings
	db.db[ID] = u

	return db.save()
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
