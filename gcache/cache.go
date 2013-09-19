package gcache

import (
	"errors"
	"github.com/zyxar/tcache"
)

type Cache struct {
	normal, expired, deleted tcache.Cache
}

func New() *Cache {
	return &Cache{tcache.New(), tcache.New(), tcache.New()}
}

func (c *Cache) Push(group string, kv tcache.KeyValue) error {
	var cache tcache.Cache
	switch group {
	case "normal":
		cache = c.normal
	case "expired":
		cache = c.expired
	case "deleted":
		cache = c.deleted
	default:
		return errors.New("unkown group: " + group)
	}
	cache.Push(kv)
	return nil
}

func (c *Cache) Pull(group, key string) (interface{}, error) {
	var cache tcache.Cache
	switch group {
	case "normal":
		cache = c.normal
	case "expired":
		cache = c.expired
	case "deleted":
		cache = c.deleted
	default:
		return nil, errors.New("unkown group: " + group)
	}
	return cache.Pull(key), nil
}

func (c *Cache) Range(group string) ([]interface{}, error) {
	var cache tcache.Cache
	switch group {
	case "normal":
		cache = c.normal
	case "expired":
		cache = c.expired
	case "deleted":
		cache = c.deleted
	default:
		return nil, errors.New("unkown group: " + group)
	}
	return cache.Range(), nil
}

func (c *Cache) Invalidate(group, key string) error {
	var cache tcache.Cache
	switch group {
	case "normal":
		cache = c.normal
	case "expired":
		cache = c.expired
	case "deleted":
		cache = c.deleted
	default:
		return errors.New("unkown group: " + group)
	}
	cache.Invalidate(key)
	return nil
}

func (c *Cache) Rebase(group, key string) (interface{}, error) {
	var cache tcache.Cache
	switch group {
	case "normal":
		cache = c.normal
	case "expired":
		cache = c.expired
	case "deleted":
		cache = c.deleted
	default:
		return nil, errors.New("unkown group: " + group)
	}
	return cache.Rebase(key), nil
}
