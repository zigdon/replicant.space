package models

import (
	"fmt"

	"encoding/json"

	"github.com/zigdon/rsp/cache"
)

var db *cache.Cache

func Parse[T any](data []byte) (*T, error) {
	s := new(T)

	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("Error parsing %T: %v", s, err)
	}

	return s, nil
}

func ConnectDB(cdb *cache.Cache) {
	db = cdb
}

type CodeAlias struct {
	orig  string
	alias string
}

func (a *CodeAlias) UnmarshalJSON(data []byte) error {
	var code string
	if err := json.Unmarshal(data, &code); err != nil {
		return err
	}
	if db == nil {
		// No database, just return this unmodified.
		*a = CodeAlias{orig: code}
		return nil
	}

	alias, err := db.Alias(code, "")
	if err != nil {
		return err
	}
	*a = CodeAlias{orig: code, alias: alias}

	return nil
}

func (a *CodeAlias) String() string {
	if a != nil {
		return a.orig
	}
	return ""
}
