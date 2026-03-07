package main

import (
	"testing"
)

func TestHash(t *testing.T) {
	skip(t)
	t.Run("basics", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("HSET", "aap", "noot", "mies")
			c.Do("HGET", "aap", "noot")
			c.Do("HMGET", "aap", "noot")
			c.Do("HLEN", "aap")
			c.Do("HKEYS", "aap")
			c.Do("HVALS", "aap")
			c.Do("HSET", "aaa", "bb", "1", "cc", "2")
			c.Do("HGET", "aaa", "bb")
			c.Do("HGET", "aaa", "cc")

			c.Do("HDEL", "aap", "noot")
			c.Do("HGET", "aap", "noot")
			c.Do("EXISTS", "aap") // key is gone

			// failure cases
			c.Error("wrong number", "HSET", "aap", "noot")
			c.Error("wrong number", "HGET", "aap")
			c.Error("wrong number", "HMGET", "aap")
			c.Error("wrong number", "HLEN")
			c.Error("wrong number", "HKEYS")
			c.Error("wrong number", "HVALS")
			c.Do("SET", "str", "I am a string")
			c.Error("wrong kind", "HSET", "str", "noot", "mies")
			c.Error("wrong kind", "HGET", "str", "noot")
			c.Error("wrong kind", "HMGET", "str", "noot")
			c.Error("wrong kind", "HLEN", "str")
			c.Error("wrong kind", "HKEYS", "str")
			c.Error("wrong kind", "HVALS", "str")
			c.Error("wrong number", "HSET")
			c.Error("wrong number", "HSET", "a1")
			c.Error("wrong number", "HSET", "a1", "b")
			c.Error("wrong number", "HSET", "a2", "b", "c", "d")
		})
	})

	t.Run("tx", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("MULTI")
			c.Do("HSET", "aap", "noot", "mies", "vuur", "wim")
			c.Do("EXEC")

			c.Do("MULTI")
			c.Do("HSET", "aap", "noot", "mies", "vuur") // uneven arg count
			c.Do("EXEC")
		})
	})

	t.Run("expire", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("HSET", "aap", "noot", "mies")
			c.Do("HEXPIRE", "aap", "3", "FIELDS", "2", "noot", "vuur")

			c.Error("wrong number", "HEXPIRE", "aap", "3", "FIELDS", "0")
			c.Error("wrong number", "HEXPIRE", "aap", "3")
			c.Error("wrong number", "HEXPIRE", "aap", "3", "FIELDS")
			c.Error("wrong number", "HEXPIRE", "aap", "-3", "FIELDS", "0")
			c.Error("wrong number", "HEXPIRE", "aap", "noot", "3")
			c.Error("not an int", "HEXPIRE", "aap", "3.14", "FIELDS", "noot", "3.14")
			c.Error("numfields", "HEXPIRE", "aap", "3", "FIELDS", "3", "noot", "vuur")
		})
	})
}

func TestHashSetnx(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("HSETNX", "aap", "noot", "mies")
		c.Do("EXISTS", "aap")
		c.Do("HEXISTS", "aap", "noot")

		c.Do("HSETNX", "aap", "noot", "mies2")
		c.Do("HGET", "aap", "noot")

		// failure cases
		c.Error("wrong number", "HSETNX", "aap")
		c.Error("wrong number", "HSETNX", "aap", "noot")
		c.Error("wrong number", "HSETNX", "aap", "noot", "too", "many")
	})
}

func TestHashDelExists(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("HSET", "aap", "noot", "mies")
		c.Do("HSET", "aap", "vuur", "wim")
		c.Do("HEXISTS", "aap", "noot")
		c.Do("HEXISTS", "aap", "vuur")
		c.Do("HDEL", "aap", "noot")
		c.Do("HEXISTS", "aap", "noot")
		c.Do("HEXISTS", "aap", "vuur")

		c.Do("HEXISTS", "nosuch", "vuur")

		// failure cases
		c.Error("wrong number", "HDEL")
		c.Error("wrong number", "HDEL", "aap")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "HDEL", "str", "key")

		c.Error("wrong number", "HEXISTS")
		c.Error("wrong number", "HEXISTS", "aap")
		c.Error("wrong number", "HEXISTS", "aap", "too", "many")
		c.Error("wrong kind", "HEXISTS", "str", "field")
	})
}

func TestHashGetall(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("HSET", "aap", "noot", "mies")
		c.Do("HSET", "aap", "vuur", "wim")
		c.DoSorted("HGETALL", "aap")

		c.Do("HGETALL", "nosuch")

		// failure cases
		c.Error("wrong number", "HGETALL")
		c.Error("wrong number", "HGETALL", "too", "many")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "HGETALL", "str")
	})

	testRESP3(t, func(c *client) {
		c.Do("HSET", "aap", "noot", "mies")
		c.Do("HGETALL", "aap")
		c.Do("HSET", "aap", "vuur", "wim")
		c.DoSorted("HGETALL", "aap")

		c.Do("HGETALL", "nosuch")
	})
}

func TestHmset(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("HMSET", "aap", "noot", "mies", "vuur", "zus")
		c.Do("HGET", "aap", "noot")
		c.Do("HGET", "aap", "vuur")
		c.Do("HLEN", "aap")

		// failure cases
		c.Error("wrong number", "HMSET", "aap")
		c.Error("wrong number", "HMSET", "aap", "key")
		c.Error("wrong number", "HMSET", "aap", "key", "value", "odd")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "HMSET", "str", "key", "value")
	})
}

func TestHashIncr(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("HINCRBY", "aap", "noot", "12")
		c.Do("HINCRBY", "aap", "noot", "-13")
		c.Do("HINCRBY", "aap", "noot", "2123")
		c.Do("HGET", "aap", "noot")

		// Simple failure cases.
		c.Error("wrong number", "HINCRBY")
		c.Error("wrong number", "HINCRBY", "aap")
		c.Error("wrong number", "HINCRBY", "aap", "noot")
		c.Error("not an integer", "HINCRBY", "aap", "noot", "noint")
		c.Error("wrong number", "HINCRBY", "aap", "noot", "12", "toomany")
		c.Do("SET", "str", "value")
		c.Error("wrong kind", "HINCRBY", "str", "value", "12")
		c.Do("HINCRBY", "aap", "noot", "12")
	})

	testRaw(t, func(c *client) {
		c.Do("HINCRBYFLOAT", "aap", "noot", "12.3")
		c.Do("HINCRBYFLOAT", "aap", "noot", "-13.1")
		c.Do("HINCRBYFLOAT", "aap", "noot", "200")
		c.Do("HGET", "aap", "noot")

		// Simple failure cases.
		c.Error("wrong number", "HINCRBYFLOAT")
		c.Error("wrong number", "HINCRBYFLOAT", "aap")
		c.Error("wrong number", "HINCRBYFLOAT", "aap", "noot")
		c.Error("not a valid float", "HINCRBYFLOAT", "aap", "noot", "noint")
		c.Error("wrong number", "HINCRBYFLOAT", "aap", "noot", "12", "toomany")
		c.Do("SET", "str", "value")
		c.Error("wrong kind", "HINCRBYFLOAT", "str", "value", "12")
		c.Do("HINCRBYFLOAT", "aap", "noot", "12")
	})
}

func TestHscan(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		// No set yet
		c.Do("HSCAN", "h", "0")

		c.Do("HSET", "h", "key1", "value1")
		c.Do("HSCAN", "h", "0")
		c.Do("HSCAN", "h", "0", "COUNT", "12")
		c.Do("HSCAN", "h", "0", "cOuNt", "12")

		c.Do("HSET", "h", "anotherkey", "value2")
		c.Do("HSCAN", "h", "0", "MATCH", "anoth*")
		c.Do("HSCAN", "h", "0", "MATCH", "anoth*", "COUNT", "100")
		c.Do("HSCAN", "h", "0", "COUNT", "100", "MATCH", "anoth*")

		// Can't really test multiple keys.
		// c.Do("SET", "key2", "value2")
		// c.Do("SCAN", "0")

		// Error cases
		c.Error("wrong number", "HSCAN")
		c.Error("wrong number", "HSCAN", "noint")
		c.Error("not an integer", "HSCAN", "h", "0", "COUNT", "noint")
		c.Error("syntax error", "HSCAN", "h", "0", "COUNT")
		c.Error("syntax error", "HSCAN", "h", "0", "MATCH")
		c.Error("syntax error", "HSCAN", "h", "0", "garbage")
		c.Error("syntax error", "HSCAN", "h", "0", "COUNT", "12", "MATCH", "foo", "garbage")
		// c.Do("HSCAN", "nosuch", "0", "COUNT", "garbage")
		c.Do("SET", "str", "1")
		c.Error("wrong kind", "HSCAN", "str", "0")
	})
}

func TestHstrlen(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("HSTRLEN", "hash", "foo")
		c.Do("HSET", "hash", "foo", "bar")
		c.Do("HSTRLEN", "hash", "foo")
		c.Do("HSTRLEN", "hash", "nosuch")
		c.Do("HSTRLEN", "nosuch", "nosuch")

		c.Error("wrong number", "HSTRLEN")
		c.Error("wrong number", "HSTRLEN", "foo")
		c.Error("wrong number", "HSTRLEN", "foo", "baz", "bar")
		c.Do("SET", "str", "1")
		c.Error("wrong kind", "HSTRLEN", "str", "bar")
	})
}

func TestHrandfield(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("HSET", "one", "foo", "bar")
		c.Do("HRANDFIELD", "one")
		c.Do("HRANDFIELD", "one", "0")
		c.Do("HRANDFIELD", "one", "1")
		c.Do("HRANDFIELD", "one", "2") // limited to 1
		c.Do("HRANDFIELD", "one", "3") // limited to 1
		c.Do("HRANDFIELD", "one", "-1")
		c.Do("HRANDFIELD", "one", "-2") // padded
		c.Do("HRANDFIELD", "one", "-3") // padded

		c.Do("HSET", "more", "foo", "bar", "baz", "bak")
		c.DoLoosely("HRANDFIELD", "more")
		c.Do("HRANDFIELD", "more", "0")
		c.DoLoosely("HRANDFIELD", "more", "1")
		c.DoLoosely("HRANDFIELD", "more", "2")
		c.DoLoosely("HRANDFIELD", "more", "3") // limited to 2
		c.DoLoosely("HRANDFIELD", "more", "-1")
		c.DoLoosely("HRANDFIELD", "more", "-2")
		c.DoLoosely("HRANDFIELD", "more", "-3") // length padded to 3

		c.Do("HRANDFIELD", "nosuch", "1")
		c.Do("HRANDFIELD", "nosuch", "2")
		c.Do("HRANDFIELD", "nosuch", "3")
		c.Do("HRANDFIELD", "nosuch", "0")
		c.Do("HRANDFIELD", "nosuch")
		c.Do("HRANDFIELD", "nosuch", "-1") // still empty
		c.Do("HRANDFIELD", "nosuch", "-2") // still empty
		c.Do("HRANDFIELD", "nosuch", "-3") // still empty
		c.DoLoosely("HRANDFIELD", "one", "2")
		c.DoLoosely("HRANDFIELD", "one", "7")
		c.DoLoosely("HRANDFIELD", "one", "2", "WITHVALUE")
		c.DoLoosely("HRANDFIELD", "one", "7", "WITHVALUE")

		c.Error("ERR syntax error", "HRANDFIELD", "foo", "1", "2")
		c.Error("ERR wrong number", "HRANDFIELD")
	})
}
