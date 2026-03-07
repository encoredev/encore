package main

import (
	"testing"
)

func TestTx(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("SET", "AAP", "1")
		c.Do("GET", "AAP")
		c.Do("EXEC")
		c.Do("GET", "AAP")
	})

	// empty
	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("EXEC")
	})

	// err: Double MULTI
	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Error("nested", "MULTI")
	})

	// err: No MULTI
	testRaw(t, func(c *client) {
		c.Error("without MULTI", "EXEC")
	})

	// Errors in the MULTI sequence
	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("SET", "foo", "bar")
		c.Error("wrong number", "SET", "foo")
		c.Do("SET", "foo", "bar")
		c.Error("EXECABORT", "EXEC")
	})

	// Simple WATCH
	testRaw(t, func(c *client) {
		c.Do("SET", "foo", "bar")
		c.Do("WATCH", "foo")
		c.Do("MULTI")
		c.Do("GET", "foo")
		c.Do("EXEC")
	})

	// Simple UNWATCH
	testRaw(t, func(c *client) {
		c.Do("SET", "foo", "bar")
		c.Do("WATCH", "foo")
		c.Do("UNWATCH")
		c.Do("MULTI")
		c.Do("GET", "foo")
		c.Do("EXEC")
	})

	// UNWATCH in a MULTI. Yep. Weird.
	testRaw(t, func(c *client) {
		c.Do("WATCH", "foo")
		c.Do("MULTI")
		c.Do("UNWATCH") // Valid. Somehow.
		c.Do("EXEC")
	})

	// Test whether all these commands support transactions.
	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("GET", "str")
		c.Do("GETEX", "str")
		c.Do("SET", "str", "bar")
		c.Do("SETNX", "str", "bar")
		c.Do("GETSET", "str", "bar")
		c.Do("MGET", "str", "bar")
		c.Do("MSET", "str", "bar")
		c.Do("MSETNX", "str", "bar")
		c.Do("SETEX", "str", "12", "newv")
		c.Do("PSETEX", "str", "12", "newv")
		c.Do("STRLEN", "str")
		c.Do("APPEND", "str", "more")
		c.Do("GETRANGE", "str", "0", "2")
		c.Do("SETRANGE", "str", "0", "B")
		c.Do("EXEC")
		c.Do("GET", "str")
	})

	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("SET", "bits", "\xff\x00")
		c.Do("BITCOUNT", "bits")
		c.Do("BITOP", "OR", "bits", "bits", "nosuch")
		c.Do("BITPOS", "bits", "1")
		c.Do("GETBIT", "bits", "12")
		c.Do("SETBIT", "bits", "12", "1")
		c.Do("EXEC")
		c.Do("GET", "bits")
	})

	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("INCR", "number")
		c.Do("INCRBY", "number", "12")
		c.Do("INCRBYFLOAT", "number", "12.2")
		c.Do("DECR", "number")
		c.Do("GET", "number")
		c.Do("DECRBY", "number", "2")
		c.Do("GET", "number")
	})

	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("HSET", "hash", "foo", "bar")
		c.Do("HDEL", "hash", "foo")
		c.Do("HEXISTS", "hash", "foo")
		c.Do("HSET", "hash", "foo", "bar22")
		c.Do("HSETNX", "hash", "foo", "bar22")
		c.Do("HGET", "hash", "foo")
		c.Do("HMGET", "hash", "foo", "baz")
		c.Do("HLEN", "hash")
		c.Do("HGETALL", "hash")
		c.Do("HKEYS", "hash")
		c.Do("HVALS", "hash")
	})

	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("SET", "key", "foo")
		c.Do("TYPE", "key")
		c.Do("EXPIRE", "key", "12")
		c.Do("TTL", "key")
		c.Do("PEXPIRE", "key", "12")
		c.Do("PTTL", "key")
		c.Do("PERSIST", "key")
		c.Do("DEL", "key")
		c.Do("TYPE", "key")
		c.Do("EXEC")
	})

	// BITOP OPs are checked after the transaction.
	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("BITOP", "BROKEN", "str", "")
		c.Do("EXEC")
	})

	// fail on invalid command
	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Error("wrong number", "GET")
		c.Error("Transaction discarded", "EXEC")
	})

	/* FIXME
		// fail on unknown command
	testRaw(t, func(c *client) {
			c.Do("MULTI")
			c.Do("NOSUCH")
			c.Do("EXEC")
	})
	*/

	// failed EXEC cleaned up the tx
	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Error("wrong number", "GET")
		c.Error("Transaction discarded", "EXEC")
		c.Do("MULTI")
	})

	testRaw2(t, func(c1, c2 *client) {
		c1.Do("WATCH", "foo")
		c1.Do("MULTI")
		c2.Do("SET", "foo", "12")
		c2.Error("without", "EXEC") // nil
		c1.Do("EXEC")               // 0-length
	})
}
