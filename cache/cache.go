package cache

import (
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
)

const BUCKET_NAME = "cache"

type Cache interface {
	Get(key string) (string, bool)
	Put(key, value string)
	Contains(key string)
	Len() int
}

// BoltCache is a persistent cache that uses BoltDB as the backend.
type BoltCache struct {
	db *bolt.DB
}

// NewBoltCache creates a new BoltCache instance with the given path.
// It is up to the caller to close the database when it is no longer needed.
func NewBoltCache(path string) (*BoltCache, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	// Create "cache" bucket if it doesn't exist
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BUCKET_NAME))
		return err
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to create default bucket")
	}

	return &BoltCache{
		db: db,
	}, nil
}

func (c *BoltCache) Get(key string) (value string, exists bool) {
	c.db.View(func(tx *bolt.Tx) error {
		val := tx.Bucket([]byte(BUCKET_NAME)).Get([]byte(key))
		if val != nil {
			value = string(val)
			exists = true
		} else {
			exists = false
		}

		return nil
	})

	return
}

func (c *BoltCache) Put(key, value string) error {
	return c.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(BUCKET_NAME)).Put([]byte(key), []byte(value))
	})
}

func (c *BoltCache) Contains(key string) bool {
	_, exists := c.Get(key)
	return exists
}

func (c *BoltCache) Len() int {
	var count int
	c.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BUCKET_NAME))
		count = b.Stats().KeyN
		return nil
	})

	return count
}

// Close closes the database.
func (c *BoltCache) Close() error {
	return c.db.Close()
}
