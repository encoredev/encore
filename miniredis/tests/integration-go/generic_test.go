package main

import (
	"strings"
	"testing"
	"time"
)

func TestKeys(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "one", "1")
		c.Do("SET", "two", "2")
		c.Do("SET", "three", "3")
		c.Do("SET", "four", "4")
		c.DoSorted("KEYS", `*o*`)
		c.DoSorted("KEYS", `t??`)
		c.DoSorted("KEYS", `t?*`)
		c.DoSorted("KEYS", `*`)
		c.DoSorted("KEYS", `t*`)
		c.DoSorted("KEYS", `t\*`)
		c.DoSorted("KEYS", `[tf]*`)

		// zero length key
		c.Do("SET", "", "nothing")
		c.Do("GET", "")

		// Simple failure cases
		c.Error("wrong number", "KEYS")
		c.Error("wrong number", "KEYS", "foo", "bar")
	})

	testRaw(t, func(c *client) {
		c.Do("SET", "[one]", "1")
		c.Do("SET", "two", "2")
		c.DoSorted("KEYS", `[\[o]*`)
		c.DoSorted("KEYS", `\[*`)
		c.DoSorted("KEYS", `*o*`)
		c.DoSorted("KEYS", `[]*`) // nothing
	})
}

func TestRandom(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("RANDOMKEY")
		// A random key from a DB with a single key. We can test that.
		c.Do("SET", "one", "1")
		c.Do("RANDOMKEY")

		// Simple failure cases
		c.Error("wrong number", "RANDOMKEY", "bar")
	})
}

func TestUnknownCommand(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Error("unknown", "nosuch")
		c.Error("unknown", "noSUCH")
		c.Error("unknown", "noSUCH", "1", "2", "3")
	})
}

func TestQuit(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("QUIT")
	})
}

func TestExists(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "a", "3")
		c.Do("HSET", "b", "c", "d")
		c.Do("EXISTS", "a", "b")
		c.Do("EXISTS", "a", "b", "q")
		c.Do("EXISTS", "a", "b", "b", "b", "a", "q")

		// Error cases
		c.Error("wrong number", "EXISTS")
	})
}

func TestRename(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		// No 'a' key
		c.Error("no such", "RENAME", "a", "b")

		// Move a key with the TTL.
		c.Do("SET", "a", "3")
		c.Do("EXPIRE", "a", "123")
		c.Do("SET", "b", "12")
		c.Do("RENAME", "a", "b")
		c.Do("EXISTS", "a")
		c.Do("GET", "a")
		c.Do("TYPE", "a")
		c.Do("TTL", "a")
		c.Do("EXISTS", "b")
		c.Do("GET", "b")
		c.Do("TYPE", "b")
		c.Do("TTL", "b")

		// move a key without TTL
		c.Do("SET", "nottl", "3")
		c.Do("RENAME", "nottl", "stillnottl")
		c.Do("TTL", "nottl")
		c.Do("TTL", "stillnottl")

		// Error cases
		c.Error("wrong number", "RENAME")
		c.Error("wrong number", "RENAME", "a")
		c.Error("wrong number", "RENAME", "a", "b", "toomany")
	})
}

func TestRenamenx(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		// No 'a' key
		c.Error("no such", "RENAMENX", "a", "b")

		c.Do("SET", "a", "value")
		c.Do("SET", "str", "value")
		c.Do("RENAMENX", "a", "str")
		c.Do("EXISTS", "a")
		c.Do("EXISTS", "str")
		c.Do("GET", "a")
		c.Do("GET", "str")

		c.Do("RENAMENX", "a", "nosuch")
		c.Do("EXISTS", "a")
		c.Do("EXISTS", "nosuch")

		// Error cases
		c.Error("wrong number", "RENAMENX")
		c.Error("wrong number", "RENAMENX", "a")
		c.Error("wrong number", "RENAMENX", "a", "b", "toomany")
	})
}

func TestScan(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		// No keys yet
		c.Do("SCAN", "0")

		c.Do("SET", "key", "value")
		c.Do("SCAN", "0")
		c.Do("SCAN", "0", "COUNT", "12")
		c.Do("SCAN", "0", "cOuNt", "12")

		c.Do("SET", "anotherkey", "value")
		c.Do("SCAN", "0", "MATCH", "anoth*")
		c.Do("SCAN", "0", "MATCH", "anoth*", "COUNT", "100")
		c.Do("SCAN", "0", "COUNT", "100", "MATCH", "anoth*")

		c.Do("SADD", "setkey", "setitem")
		c.Do("SCAN", "0", "TYPE", "set")
		c.Do("SCAN", "0", "tYpE", "sEt")
		c.Do("SCAN", "0", "TYPE", "not-a-type")
		c.Do("SCAN", "0", "TYPE", "set", "MATCH", "setkey")
		c.Do("SCAN", "0", "TYPE", "set", "COUNT", "100")
		c.Do("SCAN", "0", "TYPE", "set", "MATCH", "setkey", "COUNT", "100")

		// SCAN may return a higher count of items than requested (See https://redis.io/docs/manual/keyspace/), so we must query all items.
		c.DoLoosely("SCAN", "0", "COUNT", "100") // cursor differs

		// Can't really test multiple keys.
		// c.Do("SET", "key2", "value2")
		// c.Do("SCAN", "0")

		// Error cases
		c.Error("wrong number", "SCAN")
		c.Error("invalid cursor", "SCAN", "noint")
		c.Error("not an integer", "SCAN", "0", "COUNT", "noint")
		c.Error("syntax error", "SCAN", "0", "COUNT", "0")
		c.Error("syntax error", "SCAN", "0", "COUNT")
		c.Error("syntax error", "SCAN", "0", "MATCH")
		c.Error("syntax error", "SCAN", "0", "garbage")
		c.Error("syntax error", "SCAN", "0", "COUNT", "12", "MATCH", "foo", "garbage")
		c.Error("syntax error", "SCAN", "0", "TYPE")
	})
}

func TestFastForward(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "key1", "value")
		c.Do("SET", "key", "value", "PX", "100")
		c.DoSorted("KEYS", "*")
		c.miniredis.FastForward(200 * time.Millisecond)
		c.real.Do("MINIREDIS.FASTFORWARD", "200")
		c.DoSorted("KEYS", "*")
	})

	testRaw(t, func(c *client) {
		c.Error("invalid expire", "SET", "key1", "value", "PX", "-100")
		c.Error("invalid expire", "SET", "key2", "value", "EX", "-100")
		c.Error("invalid expire", "SET", "key3", "value", "EX", "0")
		c.DoSorted("KEYS", "*")

		c.Do("SET", "key4", "value")
		c.DoSorted("KEYS", "*")
		c.Do("EXPIRE", "key4", "-100")
		c.DoSorted("KEYS", "*")

		c.Do("SET", "key4", "value")
		c.DoSorted("KEYS", "*")
		c.Do("EXPIRE", "key4", "0")
		c.DoSorted("KEYS", "*")
	})
}

func TestProto(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ECHO", strings.Repeat("X", 1<<24))
	})
}

func TestSwapdb(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "key1", "val1")
		c.Do("SWAPDB", "0", "1")
		c.Do("SELECT", "1")
		c.Do("GET", "key1")

		c.Do("SWAPDB", "1", "1")
		c.Do("GET", "key1")

		c.Error("wrong number", "SWAPDB")
		c.Error("wrong number", "SWAPDB", "1")
		c.Error("wrong number", "SWAPDB", "1", "2", "3")
		c.Error("invalid first", "SWAPDB", "foo", "2")
		c.Error("invalid second", "SWAPDB", "1", "bar")
		c.Error("invalid first", "SWAPDB", "foo", "bar")
		c.Error("out of range", "SWAPDB", "-1", "2")
		c.Error("out of range", "SWAPDB", "1", "-2")
		// c.Do("SWAPDB", "1", "1000") // miniredis has no upperlimit
	})

	// SWAPDB with transactions
	testRaw2(t, func(c1, c2 *client) {
		c1.Do("SET", "foo", "foooooo")

		c1.Do("MULTI")
		c1.Do("SWAPDB", "0", "2")
		c1.Do("GET", "foo")
		c2.Do("GET", "foo")

		c1.Do("EXEC")
		c1.Do("GET", "foo")
		c2.Do("GET", "foo")
	})
}

func TestDel(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "one", "1")
		c.Do("SET", "two", "2")
		c.Do("SET", "three", "3")
		c.Do("SET", "four", "4")
		c.Do("DEL", "one")
		c.DoSorted("KEYS", "*")

		c.Do("DEL", "twoooo")
		c.DoSorted("KEYS", "*")

		c.Do("DEL", "two", "four")
		c.DoSorted("KEYS", "*")

		c.Error("wrong number", "DEL")
		c.DoSorted("KEYS", "*")
	})
}

func TestUnlink(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "one", "1")
		c.Do("SET", "two", "2")
		c.Do("SET", "three", "3")
		c.Do("SET", "four", "4")
		c.Do("UNLINK", "one")
		c.DoSorted("KEYS", "*")

		c.Do("UNLINK", "twoooo")
		c.DoSorted("KEYS", "*")

		c.Do("UNLINK", "two", "four")
		c.DoSorted("KEYS", "*")

		c.Error("wrong number", "UNLINK")
		c.DoSorted("KEYS", "*")
	})
}

func TestTouch(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "a", "some value")
		c.Do("TOUCH", "a")
		c.Do("GET", "a")
		c.Do("TTL", "a")

		c.Do("TOUCH", "a", "foobar", "a")

		c.Error("wrong number", "TOUCH")
	})
}

func TestPersist(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "foo", "bar")
		c.Do("EXPIRE", "foo", "12")
		c.Do("TTL", "foo")
		c.Do("PERSIST", "foo")
		c.Do("TTL", "foo")
	})
}

func TestCopy(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Error("wrong number", "COPY")
		c.Error("wrong number", "COPY", "a")
		c.Error("syntax", "COPY", "a", "b", "c")
		c.Error("syntax", "COPY", "a", "b", "DB")
		c.Error("range", "COPY", "a", "b", "DB", "-1")
		c.Error("integer", "COPY", "a", "b", "DB", "foo")
		c.Error("syntax", "COPY", "a", "b", "DB", "1", "REPLACE", "foo")

		c.Do("SET", "a", "1")
		c.Do("COPY", "a", "b") // returns 1 - successfully copied
		c.Do("EXISTS", "b")
		c.Do("GET", "b")
		c.Do("TYPE", "b")

		c.Do("COPY", "nonexistent", "c") // returns 1 - not successfully copied
		c.Do("RENAME", "b", "c")         // rename the copied key

		t.Run("replace option", func(t *testing.T) {
			c.Do("SET", "fromme", "1")
			c.Do("HSET", "replaceme", "foo", "bar")
			c.Do("COPY", "fromme", "replaceme", "REPLACE")
			c.Do("TYPE", "replaceme")
			c.Do("GET", "replaceme")
		})

		t.Run("different DB", func(t *testing.T) {
			c.Do("SELECT", "2")
			c.Do("SET", "fromme", "1")
			c.Do("COPY", "fromme", "replaceme", "DB", "3")
			c.Do("EXISTS", "replaceme") // your value is in another db
			c.Do("SELECT", "3")
			c.Do("EXISTS", "replaceme")
			c.Do("TYPE", "replaceme")
			c.Do("GET", "replaceme")
		})
		c.Do("SELECT", "0")

		t.Run("copy to self", func(t *testing.T) {
			// copy to self is never allowed
			c.Do("SET", "double", "1")
			c.Error("the same", "COPY", "double", "double")
			c.Error("the same", "COPY", "double", "double", "REPLACE")
			c.Do("COPY", "double", "double", "DB", "2") // different DB is fine
			c.Do("SELECT", "2")
			c.Do("TYPE", "double")

			c.Error("the same", "COPY", "noexisting", "noexisting") // "copy to self?" check comes before key check
		})
		c.Do("SELECT", "0")

		// deep copies?
		t.Run("hash", func(t *testing.T) {
			c.Do("HSET", "temp", "paris", "12")
			c.Do("HSET", "temp", "oslo", "-5")
			c.Do("COPY", "temp", "temp2")
			c.Do("TYPE", "temp2")
			c.Do("HGET", "temp2", "oslo")
			c.Do("HSET", "temp2", "oslo", "-7")
			c.Do("HGET", "temp", "oslo")
			c.Do("HGET", "temp2", "oslo")
		})

		t.Run("list set", func(t *testing.T) {
			c.Do("LPUSH", "list", "aap", "noot", "mies")
			c.Do("COPY", "list", "list2")
			c.Do("TYPE", "list2")
			c.Do("LSET", "list", "1", "vuur")
			c.Do("LRANGE", "list", "0", "-1")
			c.Do("LRANGE", "list2", "0", "-1")
		})

		t.Run("list", func(t *testing.T) {
			c.Do("LPUSH", "list", "aap", "noot", "mies")
			c.Do("COPY", "list", "list2")
			c.Do("TYPE", "list2")
			c.Do("LPUSH", "list", "vuur")
			c.Do("LRANGE", "list", "0", "-1")
			c.Do("LRANGE", "list2", "0", "-1")
		})

		t.Run("set", func(t *testing.T) {
			c.Do("SADD", "set", "aap", "noot", "mies")
			c.Do("COPY", "set", "set2")
			c.Do("TYPE", "set2")
			c.DoSorted("SMEMBERS", "set2")
			c.Do("SADD", "set", "vuur")
			c.DoSorted("SMEMBERS", "set")
			c.DoSorted("SMEMBERS", "set2")
		})

		t.Run("sorted set", func(t *testing.T) {
			c.Do("ZADD", "zset", "1", "aap", "2", "noot", "3", "mies")
			c.Do("COPY", "zset", "zset2")
			c.Do("TYPE", "zset2")
			c.Do("ZCARD", "zset")
			c.Do("ZCARD", "zset2")
			c.Do("ZADD", "zset", "4", "vuur")
			c.Do("ZCARD", "zset")
			c.Do("ZCARD", "zset2")
		})

		t.Run("stream", func(t *testing.T) {
			c.Do("XADD",
				"planets",
				"0-1",
				"name", "Mercury",
			)
			c.Do("COPY", "planets", "planets2")
			c.Do("XLEN", "planets2")
			c.Do("TYPE", "planets2")

			c.Do("XADD",
				"planets",
				"18446744073709551000-0",
				"name", "Earth",
			)
			c.Do("XLEN", "planets")
			c.Do("XLEN", "planets2")
		})

		t.Run("stream", func(t *testing.T) {
			c.Do("PFADD", "hlog", "42")
			c.DoApprox(2, "PFCOUNT", "hlog")
			c.Do("COPY", "hlog", "hlog2")
			// c.Do("TYPE", "hlog2") broken
			c.Do("PFADD", "hlog", "44")
			c.Do("PFCOUNT", "hlog")
			c.Do("PFCOUNT", "hlog2")
		})
	})
}

func TestClient(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		// Try to get the client name without setting it first
		c.Do("CLIENT", "GETNAME")

		c.Do("CLIENT", "SETNAME", "miniredis-tests")
		c.Do("CLIENT", "GETNAME")
		c.Do("CLIENT", "SETNAME", "miniredis-tests2")
		c.Do("CLIENT", "GETNAME")
		c.Do("CLIENT", "SETNAME", "")
		c.Do("CLIENT", "GETNAME")

		c.Error("wrong number", "CLIENT")
		c.Error("unknown subcommand", "CLIENT", "FOOBAR")
		c.Error("wrong number", "CLIENT", "GETNAME", "foo")
		c.Error("contain spaces", "CLIENT", "SETNAME", "miniredis tests")
		c.Error("contain spaces", "CLIENT", "SETNAME", "miniredis\ntests")
	})

	testRaw2(t, func(c1, c2 *client) {
		c1.Do("MULTI")
		c1.Do("CLIENT", "SETNAME", "conn-c1")
		c1.Do("CLIENT", "GETNAME")
		c2.Do("CLIENT", "GETNAME") // not set yet
		c1.Do("EXEC")
		c1.Do("CLIENT", "GETNAME")
		c2.Do("CLIENT", "GETNAME")
	})
}

func TestObject(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("OBJECT", "IDLETIME", "foo")

		c.Do("SET", "foo", "bar")
		c.Do("OBJECT", "IDLETIME", "foo")
		c.Do("GET", "foo")
		c.Do("OBJECT", "IDLETIME", "foo")

		c.Error("number", "OBJECT")
		c.Error("unknown subcommand 'foo'", "OBJECT", "foo")
		c.Error("object|idletime", "OBJECT", "IDLETIME")
		c.Error("wrong number", "OBJECT", "IDLETIME", "foo", "bar")

		c.Do("MULTI")
		c.Do("OBJECT", "IDLETIME", "foo")
		c.Error("object|idletime", "OBJECT", "IDLETIME", "bar", "baz")
		c.Error("object|idletime", "OBJECT", "IDLETIME")
		c.Error("Transaction discarded", "EXEC")
	})
}
