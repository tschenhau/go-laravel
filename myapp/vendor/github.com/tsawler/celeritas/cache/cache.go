package cache

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/gomodule/redigo/redis"
)

// Cache is the interface for the Cache type. Anything that
// implements caching functionality must satisfy this interface
// by implementing all of its methods
type Cache interface {
	Has(string) (bool, error)
	Get(string) (interface{}, error)
	Set(string, interface{}, ...int) error
	Forget(string) error
	Empty() error
	EmptyByMatch(string) error
}

// RedisCache holds the cache type and client/pool (if applicable)
type RedisCache struct {
	Conn   *redis.Pool
	Prefix string
}

// Entry is a map to hold values, so we can serialize them
type Entry map[string]interface{}

// Has checks to see if key exists in redis
func (c *RedisCache) Has(str string) (bool, error) {
	key := fmt.Sprintf("%s:%s", c.Prefix, str)
	conn := c.Conn.Get()
	defer conn.Close()

	ok, err := redis.Bool(conn.Do("EXISTS", key))
	if err != nil {
		return false, err
	}
	return ok, err
}

// Encode serializes item, from a map[string]interface{}
func encode(item Entry) ([]byte, error) {
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	err := e.Encode(item)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Decode unserializes item into a map[string]interface{}
func decode(str string) (Entry, error) {
	item := Entry{}
	b := bytes.Buffer{}
	b.Write([]byte(str))
	d := gob.NewDecoder(&b)
	err := d.Decode(&item)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// Get pulls an item from Redis, unserializes it, and returns it
func (c *RedisCache) Get(str string) (interface{}, error) {
	key := fmt.Sprintf("%s:%s", c.Prefix, str)
	conn := c.Conn.Get()
	defer conn.Close()

	cacheEntry, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		return nil, err
	}

	decoded, err := decode(string(cacheEntry))
	if err != nil {
		return nil, err
	}
	item := decoded[key]
	return item, nil
}

// Set serializes an object and puts it into redis with optional expires, in seconds
func (c *RedisCache) Set(str string, value interface{}, expires ...int) error {
	key := fmt.Sprintf("%s:%s", c.Prefix, str)
	conn := c.Conn.Get()
	defer conn.Close()

	entry := Entry{}
	entry[key] = value
	encoded, err := encode(entry)
	if err != nil {
		return err
	}

	if len(expires) > 0 {
		_, err = conn.Do("SETEX", key, expires[0], string(encoded))
		if err != nil {
			return err
		}
	} else {
		_, err = conn.Do("SET", key, string(encoded))
		if err != nil {
			return err
		}
	}

	return nil
}

// Forget drops an item from the cache
func (c *RedisCache) Forget(str string) error {
	key := fmt.Sprintf("%s:%s", c.Prefix, str)
	conn := c.Conn.Get()
	defer conn.Close()
	_, err := conn.Do("DEL", key)
	if err != nil {
		return err
	}
	return nil
}

// Empty removes all cached entries for this application
func (c *RedisCache) Empty() error {
	key := fmt.Sprintf("%s:", c.Prefix)
	conn := c.Conn.Get()
	defer conn.Close()

	matches, err := c.getKeys(key)
	if err != nil {
		return err
	}

	for _, x := range matches {
		err = c.Forget(x)
		_, err := conn.Do("DEL", x)
		if err != nil {
			return err
		}
	}

	return nil
}

// EmptyByMatch removes all entries in redis that match the prefix match
func (c *RedisCache) EmptyByMatch(str string) error {
	key := fmt.Sprintf("%s:%s", c.Prefix, str)
	conn := c.Conn.Get()
	defer conn.Close()

	matches, err := c.getKeys(key)
	if err != nil {
		return err
	}

	for _, x := range matches {
		err = c.Forget(x)
		_, err := conn.Do("DEL", x)
		if err != nil {
			return err
		}
	}

	return nil
}

// getKeys scans redis for all entries matching pattern*, and returns a slice of strings
func (c *RedisCache) getKeys(pattern string) ([]string, error) {
	conn := c.Conn.Get()
	defer conn.Close()

	iter := 0
	keys := []string{}
	for {
		arr, err := redis.Values(conn.Do("SCAN", iter, "MATCH", fmt.Sprintf("%s*", pattern)))
		if err != nil {
			return keys, fmt.Errorf("error retrieving '%s' keys", pattern)
		}

		iter, _ = redis.Int(arr[0], nil)
		k, _ := redis.Strings(arr[1], nil)
		keys = append(keys, k...)

		if iter == 0 {
			break
		}
	}

	return keys, nil
}
