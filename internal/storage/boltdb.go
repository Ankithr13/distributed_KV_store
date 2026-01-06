package storage

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	bolt "go.etcd.io/bbolt"

	"github.com/ankithrao/distributed-kv/internal/common"
)

const (
	bMeta = "meta"
	bLog  = "log"
	bKV   = "kv"
)

type BoltStore struct {
	db *bolt.DB
}

func Open(dir string) (*BoltStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dir, "kv.db")
	db, err := bolt.Open(dbPath, 0o600, nil)
	if err != nil {
		return nil, err
	}
	s := &BoltStore{db: db}
	if err := db.Update(func(tx *bolt.Tx) error {
		if _, e := tx.CreateBucketIfNotExists([]byte(bMeta)); e != nil {
			return e
		}
		if _, e := tx.CreateBucketIfNotExists([]byte(bLog)); e != nil {
			return e
		}
		if _, e := tx.CreateBucketIfNotExists([]byte(bKV)); e != nil {
			return e
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *BoltStore) Close() error { return s.db.Close() }

func u64key(x uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], x)
	return b[:]
}

func (s *BoltStore) GetMetaU64(key string) (uint64, bool, error) {
	var out uint64
	var ok bool
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte(bMeta)).Get([]byte(key))
		if v == nil {
			return nil
		}
		ok = true
		out = binary.BigEndian.Uint64(v)
		return nil
	})
	return out, ok, err
}

func (s *BoltStore) PutMetaU64(key string, val uint64) error {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], val)
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bMeta)).Put([]byte(key), b[:])
	})
}

func (s *BoltStore) AppendLog(e common.LogEntry) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bLog)).Put(u64key(e.Index), raw)
	})
}

func (s *BoltStore) ReadLog(index uint64) (common.LogEntry, bool, error) {
	var e common.LogEntry
	var ok bool
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte(bLog)).Get(u64key(index))
		if v == nil {
			return nil
		}
		ok = true
		return json.Unmarshal(v, &e)
	})
	return e, ok, err
}

func (s *BoltStore) LastLogIndex() (uint64, error) {
	var last uint64
	err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(bLog)).Cursor()
		k, _ := c.Last()
		if k == nil {
			last = 0
			return nil
		}
		last = binary.BigEndian.Uint64(k)
		return nil
	})
	return last, err
}

func (s *BoltStore) ApplyToKV(e common.LogEntry) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bKV))
		switch e.Op {
		case common.OpPut:
			return b.Put([]byte(e.Key), []byte(e.Value))
		case common.OpDel:
			return b.Delete([]byte(e.Key))
		default:
			return fmt.Errorf("unknown op: %s", e.Op)
		}
	})
}

func (s *BoltStore) GetKV(key string) (string, bool, error) {
	var out string
	var ok bool
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte(bKV)).Get([]byte(key))
		if v == nil {
			return nil
		}
		ok = true
		out = string(v)
		return nil
	})
	return out, ok, err
}
