package cache

import (
	"fmt"
	"github.com/dgraph-io/badger/v3"
	"time"
)

// BadgerCache is the type for badger cache.
type BadgerCache struct {
	Conn   *badger.DB
	Prefix string
}

// Has checks for existence of key str in cache, and returns true/false, and error if any
func (b *BadgerCache) Has(str string) (bool, error) {
	_, err := b.Get(str)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Get pulls an item from badger, unserializes it, and returns it
func (b *BadgerCache) Get(str string) (interface{}, error) {
	key := fmt.Sprintf("%s:%s", b.Prefix, str)
	var fromCache []byte

	err := b.Conn.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		err = item.Value(func(val []byte) error {
			fromCache = append([]byte{}, val...)
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	decoded, err := decode(string(fromCache))
	if err != nil {
		return nil, err
	}
	item := decoded[key]

	return item, nil
}

// Set serializes an object and puts it into badger with optional expires, in seconds
func (b *BadgerCache) Set(str string, value interface{}, expires ...int) error {
	key := fmt.Sprintf("%s:%s", b.Prefix, str)
	entry := Entry{}
	entry[key] = value
	encoded, err := encode(entry)
	if err != nil {
		return err
	}

	if len(expires) > 0 {
		err = b.Conn.Update(func(txn *badger.Txn) error {
			e := badger.NewEntry([]byte(key), encoded).WithTTL(time.Hour * time.Duration(expires[0]))
			err = txn.SetEntry(e)
			return err
		})
	} else {
		err = b.Conn.Update(func(txn *badger.Txn) error {
			err := txn.Set([]byte(key), encoded)
			return err
		})
	}

	return err
}

// Forget drops an entry from the cache if it exists
func (b *BadgerCache) Forget(str string) error {
	key := fmt.Sprintf("%s:%s", b.Prefix, str)
	err := b.Conn.Update(func(txn *badger.Txn) error {
		err := txn.Delete([]byte(key))
		return err
	})

	return err
}

// Empty empties the cache for all cached values for this application
func (b *BadgerCache) Empty() error {
	return b.emptyByMatch("")
}

// EmptyByMatch removes all entries in redis that match the prefix match
func (b *BadgerCache) EmptyByMatch(str string) error {
	return b.emptyByMatch(str)
}

// emptyByMatch is a helper function that drops entries that have either no prefix (str = "")
// or which begin with the prefix string (str != "")
func (b *BadgerCache) emptyByMatch(str string) error {
	prefix := fmt.Sprintf("%s:%s", b.Prefix, str)

	deleteKeys := func(keysForDelete [][]byte) error {
		if err := b.Conn.Update(func(txn *badger.Txn) error {
			for _, key := range keysForDelete {
				if err := txn.Delete(key); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	}

	collectSize := 100000

	err := b.Conn.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.AllVersions = false
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		keysForDelete := make([][]byte, 0, collectSize)
		keysCollected := 0
		for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
			key := it.Item().KeyCopy(nil)
			keysForDelete = append(keysForDelete, key)
			keysCollected++
			if keysCollected == collectSize {
				if err := deleteKeys(keysForDelete); err != nil {
					panic(err)
				}
				keysForDelete = make([][]byte, 0, collectSize)
				keysCollected = 0
			}
		}
		if keysCollected > 0 {
			if err := deleteKeys(keysForDelete); err != nil {
				panic(err)
			}
		}

		return nil
	})

	return err
}
