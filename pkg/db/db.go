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

var ErrUserNotFound = errors.New("User not found")

type DB struct {
	l    sync.RWMutex
	path string
	db   map[int64]UserData
}

type UserData struct {
	Settings UserSettings
	Levels   map[string]map[string]Level
}

type Level struct {
	Type    ValueType
	DeltaID string
}

type UserSettings struct {
	Delta float64
}

type Value struct {
	Key       string
	Value     float64
	Type      ValueType
	Precision uint8
	DeltaID   string
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
			Levels: map[string]map[string]Level{},
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
			db.db[ID].Levels[key] = map[string]Level{}
		}
		v := val.StringValue()
		if _, exists := db.db[ID].Levels[key][v]; !exists {
			db.db[ID].Levels[key][v] = Level{Type: val.Type, DeltaID: val.DeltaID}
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
	delete(db.db[ID].Levels, key)
}

func (db *DB) DeleteValue(ID int64, val Value) error {
	db.l.Lock()
	defer db.l.Unlock()
	if db.db == nil {
		return nil
	}
	if db.db[ID].Levels == nil {
		return nil
	}
	if db.db[ID].Levels[val.Key] == nil {
		return nil
	}
	v := val.StringValue()
	delete(db.db[ID].Levels[val.Key], v)
	if val.DeltaID != "" {
		delK := ""
		for k, kv := range db.db[ID].Levels[val.Key] {
			if kv.DeltaID == val.DeltaID {
				delK = k
				break
			}
		}
		if delK != "" {
			delete(db.db[ID].Levels[val.Key], delK)
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
		for rawV := range db.db[ID].Levels[k] {
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
			lst = append(
				lst,
				Value{
					Key:       k,
					Value:     v,
					Type:      db.db[ID].Levels[k][rawV].Type,
					Precision: uint8(prec),
					DeltaID:   db.db[ID].Levels[k][rawV].DeltaID,
				},
			)
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
