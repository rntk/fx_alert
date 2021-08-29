package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
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

type Levels map[string]ValueType

type Deltas map[string]float64

type SymbolSettings struct {
	Levels Levels
	Deltas Deltas
}

type DB struct {
	l    sync.RWMutex
	path string
	db   map[int64]map[string]SymbolSettings
}

type LevelValue struct {
	Key   string
	Value float64
	Type  ValueType
}

func (v LevelValue) IsAlert(currentV float64) bool {
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

func (v LevelValue) String() string {
	return fmt.Sprintf("%s %s %.5f", v.Key, v.Type, v.Value)
}

type DeltaValue struct {
	Key   string
	Value float64
	Delta float64
}

func (dv DeltaValue) IsAlert(price float64) bool {
	dlt := math.Abs(price-dv.Value) * 100000

	return dlt >= dv.Delta
}

func (v DeltaValue) String() string {
	return fmt.Sprintf("%s %.5f %.0f", v.Key, v.Value, v.Delta)
}

func (db *DB) AddLevel(ID int64, val LevelValue) error {
	db.l.Lock()
	defer db.l.Unlock()
	if db.db == nil {
		db.db = map[int64]map[string]SymbolSettings{}
	}
	if db.db[ID] == nil {
		db.db[ID] = map[string]SymbolSettings{}
	}
	sett := db.db[ID][val.Key]
	if sett.Levels == nil {
		sett.Levels = Levels{}
	}
	db.db[ID][val.Key] = sett
	v := strconv.FormatFloat(val.Value, 'f', 5, 64)
	if _, exists := db.db[ID][val.Key].Levels[v]; !exists {
		db.db[ID][val.Key].Levels[v] = val.Type
	}

	return db.save()
}

func (db *DB) AddDeltas(ID int64, values []DeltaValue) error {
	db.l.Lock()
	defer db.l.Unlock()
	if db.db == nil {
		db.db = map[int64]map[string]SymbolSettings{}
	}
	if db.db[ID] == nil {
		db.db[ID] = map[string]SymbolSettings{}
	}
	for _, val := range values {
		sett := db.db[ID][val.Key]
		if sett.Deltas == nil {
			sett.Deltas = Deltas{}
		}
		db.db[ID][val.Key] = sett
		v := strconv.FormatFloat(val.Value, 'f', 5, 64)
		if _, exists := db.db[ID][val.Key].Deltas[v]; !exists {
			db.db[ID][val.Key].Deltas[v] = val.Delta
		}
	}

	return db.save()
}

func (db *DB) DeleteSymbolKey(ID int64, key string) error {
	db.l.Lock()
	defer db.l.Unlock()
	db.deleteSymbolKey(ID, key)

	return db.save()
}

func (db *DB) deleteSymbolKey(ID int64, key string) {
	if db.db == nil {
		return
	}
	if _, exists := db.db[ID]; !exists {
		return
	}
	delete(db.db[ID], key)
	// delete empty user
	if len(db.db[ID]) == 0 {
		delete(db.db, ID)
	}
}

func (db *DB) deleteEmptySymbol(ID int64, key string) {
	if (len(db.db[ID][key].Levels) == 0) && (len(db.db[ID][key].Deltas) == 0) {
		db.deleteSymbolKey(ID, key)
	}
}

func (db *DB) DeleteLevelValue(ID int64, key string, value float64) error {
	db.l.Lock()
	defer db.l.Unlock()
	if db.db == nil {
		return nil
	}
	if db.db[ID] == nil {
		return nil
	}
	if db.db[ID][key].Levels == nil {
		return nil
	}
	v := strconv.FormatFloat(value, 'f', 5, 64)
	delete(db.db[ID][key].Levels, v)
	db.deleteEmptySymbol(ID, key)

	return db.save()
}

func (db *DB) DeleteDeltaValue(ID int64, key string, value float64) error {
	db.l.Lock()
	defer db.l.Unlock()
	if db.db == nil {
		return nil
	}
	if db.db[ID] == nil {
		return nil
	}
	if db.db[ID][key].Deltas == nil {
		return nil
	}
	v := strconv.FormatFloat(value, 'f', 5, 64)
	delete(db.db[ID][key].Deltas, v)
	db.deleteEmptySymbol(ID, key)

	return db.save()
}

func (db *DB) ListLevels(ID int64) []LevelValue {
	db.l.RLock()
	defer db.l.RUnlock()
	if db.db == nil {
		return nil
	}
	if db.db[ID] == nil {
		return nil
	}
	var lst []LevelValue
	for k := range db.db[ID] {
		for rawV := range db.db[ID][k].Levels {
			v, err := strconv.ParseFloat(rawV, 64)
			if err != nil {
				// TODO: panic or change db scheme?
				log.Printf("Can't parse float value from base: %q. %v", rawV, err)
				continue
			}
			lst = append(lst, LevelValue{Key: k, Value: v, Type: db.db[ID][k].Levels[rawV]})
		}
	}

	return lst
}

func (db *DB) ListDeltas(ID int64) []DeltaValue {
	db.l.RLock()
	defer db.l.RUnlock()
	if db.db == nil {
		return nil
	}
	if db.db[ID] == nil {
		return nil
	}
	var lst []DeltaValue
	for k := range db.db[ID] {
		for rawV := range db.db[ID][k].Deltas {
			v, err := strconv.ParseFloat(rawV, 64)
			if err != nil {
				// TODO: panic or change db scheme?
				log.Printf("Can't parse float value from base: %q. %v", rawV, err)
				continue
			}
			lst = append(lst, DeltaValue{Key: k, Value: v, Delta: db.db[ID][k].Deltas[rawV]})
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
