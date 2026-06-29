package tui

import (
	"sync"

	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

type cachable interface {
	Update() error
	Type() string
	ID() *models.CodeAlias
}

type cache struct {
	mu sync.Mutex
	c map[string]cachable
}

func newCache() *cache {
	return &cache{
		c: make(map[string]cachable),
	}
}

func (c *cache) update() error {
	acc, err := rest.Account()
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range acc.Replicants {
		c.c[r.Code.String()] = r
	}

	return nil
}

func (c *cache) getAll(t string) []cachable {
	var res []cachable
	for _, v := range c.c {
		if v.Type() != t {
			continue
		}
		res = append(res, v)
	}
	return res
}

