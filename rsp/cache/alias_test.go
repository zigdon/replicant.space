package cache

import (
	"testing"
)

func TestAliasHelpers(t *testing.T) {
	var c *Cache

	// Dealias without dash
	if got := c.Dealias("SOL"); got != "SOL" {
		t.Errorf("Dealias(\"SOL\") = %q, expected \"SOL\"", got)
	}

	// Dealias with dash on nil DB
	if got := c.Dealias("af-1"); got != "af-1" {
		t.Errorf("Dealias(\"af-1\") with nil DB = %q, expected \"af-1\"", got)
	}

	// GetAliasAndType on nil DB
	if a, typ := c.GetAliasAndType("ABC"); a != "" || typ != "" {
		t.Errorf("GetAliasAndType on nil DB expected empty strings, got (%q, %q)", a, typ)
	}

	// HasAlias on nil DB
	if got := c.HasAlias("ABC"); got != "" {
		t.Errorf("HasAlias on nil DB expected \"\", got %q", got)
	}
}
