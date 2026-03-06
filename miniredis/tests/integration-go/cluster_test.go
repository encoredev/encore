package main

import "testing"

func TestCluster(t *testing.T) {
	skip(t)
	testCluster(t,
		func(c *client) {
			// c.DoLoosly("CLUSTER", "SLOTS")
			c.DoLoosely("CLUSTER", "KEYSLOT", "{test}")
			c.DoLoosely("CLUSTER", "NODES")
			c.Error("wrong number", "CLUSTER")
			// c.DoLoosely("CLUSTER", "SHARDS")
		},
	)
}
