package kv

import (
	"sync/atomic"

	"github.com/ankithrao/distributed-kv/internal/common"
	"github.com/ankithrao/distributed-kv/internal/storage"
)

type KV struct {
	store      *storage.BoltStore
	applyIndex atomic.Uint64
}

func New(store *storage.BoltStore) *KV {
	return &KV{store: store}
}

func (k *KV) Apply(e common.LogEntry) error {
	if err := k.store.AppendLog(e); err != nil {
		return err
	}
	if err := k.store.ApplyToKV(e); err != nil {
		return err
	}
	k.applyIndex.Store(e.Index)
	return nil
}

func (k *KV) Get(key string) (string, bool, error) {
	return k.store.GetKV(key)
}

func (k *KV) AppliedIndex() uint64 { return k.applyIndex.Load() }
