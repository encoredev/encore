package main

import (
	"testing"
)

func TestSet(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SADD", "s", "aap", "noot", "mies")
		c.Do("SADD", "s", "vuur", "noot")
		c.Do("TYPE", "s")
		c.Do("EXISTS", "s")
		c.Do("SCARD", "s")
		c.DoSorted("SMEMBERS", "s")
		c.DoSorted("SMEMBERS", "nosuch")
		c.Do("SISMEMBER", "s", "aap")
		c.Do("SISMEMBER", "s", "nosuch")
		c.Do("SMISMEMBER", "s", "aap", "noot", "nosuch")

		c.Do("SCARD", "nosuch")
		c.Do("SISMEMBER", "nosuch", "nosuch")
		c.Do("SMISMEMBER", "nosuch", "nosuch", "nosuch")

		// failure cases
		c.Error("wrong number", "SADD")
		c.Error("wrong number", "SADD", "s")
		c.Error("wrong number", "SMEMBERS")
		c.Error("wrong number", "SMEMBERS", "too", "many")
		c.Error("wrong number", "SCARD")
		c.Error("wrong number", "SCARD", "too", "many")
		c.Error("wrong number", "SISMEMBER")
		c.Error("wrong number", "SISMEMBER", "few")
		c.Error("wrong number", "SISMEMBER", "too", "many", "arguments")
		c.Error("wrong number", "SMISMEMBER")
		c.Error("wrong number", "SMISMEMBER", "few")
		// Wrong type
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "SADD", "str", "noot", "mies")
		c.Error("wrong kind", "SMEMBERS", "str")
		c.Error("wrong kind", "SISMEMBER", "str", "noot")
		c.Error("wrong kind", "SMISMEMBER", "str", "noot")
		c.Error("wrong kind", "SCARD", "str")
	})

	testRESP3(t, func(c *client) {
		c.Do("SMEMBERS", "q")
		c.Do("SADD", "q", "aap")
		c.Do("SMEMBERS", "q")
		c.Do("SISMEMBER", "q", "aap")
		c.Do("SISMEMBER", "q", "noot")
		c.Do("SMISMEMBER", "q", "aap", "noot", "nosuch")
	})
}

func TestSetMove(t *testing.T) {
	skip(t)
	// Move a set around
	testRaw(t, func(c *client) {
		c.Do("SADD", "s", "aap", "noot", "mies")
		c.Do("RENAME", "s", "others")
		c.DoSorted("SMEMBERS", "s")
		c.DoSorted("SMEMBERS", "others")
		c.Do("MOVE", "others", "2")
		c.DoSorted("SMEMBERS", "others")
		c.Do("SELECT", "2")
		c.DoSorted("SMEMBERS", "others")
	})
}

func TestSetDel(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SADD", "s", "aap", "noot", "mies")
		c.Do("SREM", "s", "noot", "nosuch")
		c.Do("SCARD", "s")
		c.DoSorted("SMEMBERS", "s")

		// failure cases
		c.Error("wrong number", "SREM")
		c.Error("wrong number", "SREM", "s")
		// Wrong type
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "SREM", "str", "noot")
	})
}

func TestSetSMove(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SADD", "s", "aap", "noot", "mies")
		c.Do("SMOVE", "s", "s2", "aap")
		c.Do("SCARD", "s")
		c.Do("SCARD", "s2")
		c.Do("SMOVE", "s", "s2", "nosuch")
		c.Do("SCARD", "s")
		c.Do("SCARD", "s2")
		c.Do("SMOVE", "s", "nosuch", "noot")
		c.Do("SCARD", "s")
		c.Do("SCARD", "s2")

		c.Do("SMOVE", "s", "s2", "mies")
		c.Do("SCARD", "s")
		c.Do("EXISTS", "s")
		c.Do("SCARD", "s2")
		c.Do("EXISTS", "s2")

		c.Do("SMOVE", "s2", "s2", "mies")

		c.Do("SADD", "s5", "aap")
		c.Do("SADD", "s6", "aap")
		c.Do("SMOVE", "s5", "s6", "aap")

		// failure cases
		c.Error("wrong number", "SMOVE")
		c.Error("wrong number", "SMOVE", "s")
		c.Error("wrong number", "SMOVE", "s", "s2")
		c.Error("wrong number", "SMOVE", "s", "s2", "too", "many")
		// Wrong type
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "SMOVE", "str", "s2", "noot")
		c.Error("wrong kind", "SMOVE", "s2", "str", "noot")
	})
}

func TestSetSpop(t *testing.T) {
	skip(t)
	t.Run("without count", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("SADD", "s", "aap")
			c.Do("SPOP", "s")
			c.Do("EXISTS", "s")

			c.Do("SPOP", "nosuch")

			c.Do("SADD", "s", "aap")
			c.Do("SADD", "s", "noot")
			c.Do("SADD", "s", "mies")
			c.Do("SADD", "s", "noot")
			c.Do("SCARD", "s")
			c.DoLoosely("SMEMBERS", "s")

			c.Do("SPOP", "s", "0")

			// failure cases
			c.Error("wrong number", "SPOP")
			c.Do("SADD", "s", "aap")
			c.Error("out of range", "SPOP", "s", "s2")
			c.Error("out of range", "SPOP", "s", "-1")
			c.Error("out of range", "SPOP", "nosuch", "s2")
			// Wrong type
			c.Do("SET", "str", "I am a string")
			c.Error("wrong kind", "SPOP", "str")
		})
	})

	t.Run("with count", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("SADD", "s", "aap")
			c.Do("SADD", "s", "noot")
			c.Do("SADD", "s", "mies")
			c.Do("SADD", "s", "vuur")
			c.DoLoosely("SPOP", "s", "2")
			c.Do("EXISTS", "s")
			c.Do("SCARD", "s")

			c.DoLoosely("SPOP", "s", "200")
			c.Do("SPOP", "s", "1")
			c.Do("SPOP", "s", "0")
			c.Do("SCARD", "s")

			c.Do("SPOP", "nosuch", "1")
			c.Do("SPOP", "nosuch", "0")

			// failure cases
			c.Error("out of range", "SPOP", "foo", "one")
			c.Error("out of range", "SPOP", "foo", "-4")
		})
	})
}

func TestSetSrandmember(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		// Set with a single member...
		c.Do("SADD", "s", "aap")
		c.Do("SRANDMEMBER", "s")
		c.Do("SRANDMEMBER", "s", "1")
		c.Do("SRANDMEMBER", "s", "5")
		c.Do("SRANDMEMBER", "s", "-1")
		c.Do("SRANDMEMBER", "s", "-5")

		c.Do("SRANDMEMBER", "s", "0")
		c.Do("SRANDMEMBER", "nosuch")
		c.Do("SRANDMEMBER", "nosuch", "1")

		// failure cases
		c.Error("wrong number", "SRANDMEMBER")
		c.Error("not an integer", "SRANDMEMBER", "s", "noint")
		c.Error("syntax error", "SRANDMEMBER", "s", "1", "toomany")
		// Wrong type
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "SRANDMEMBER", "str")
	})

	testRESP3(t, func(c *client) {
		c.Do("SADD", "q", "aap")
		c.Do("SRANDMEMBER", "q")
		c.Do("SRANDMEMBER", "q", "1")
		c.Do("SRANDMEMBER", "q", "0")
	})
}

func TestSetSdiff(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SADD", "s1", "aap", "noot", "mies")
		c.Do("SADD", "s2", "noot", "mies", "vuur")
		c.Do("SADD", "s3", "mies", "wim")
		c.DoSorted("SDIFF", "s1")
		c.DoSorted("SDIFF", "s1", "s2")
		c.DoSorted("SDIFF", "s1", "s2", "s3")
		c.Do("SDIFF", "nosuch")
		c.Do("SDIFF", "s1", "nosuch", "s2", "nosuch", "s3")
		c.Do("SDIFF", "s1", "s1")

		c.Do("SDIFFSTORE", "res", "s3", "nosuch", "s1")
		c.Do("SMEMBERS", "res")

		// failure cases
		c.Error("wrong number", "SDIFF")
		c.Error("wrong number", "SDIFFSTORE")
		c.Error("wrong number", "SDIFFSTORE", "key")
		// Wrong type
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "SDIFF", "s1", "str")
		c.Error("wrong kind", "SDIFF", "nosuch", "str")
		c.Error("wrong kind", "SDIFF", "str", "s1")
		c.Error("wrong kind", "SDIFFSTORE", "res", "str", "s1")
		c.Error("wrong kind", "SDIFFSTORE", "res", "s1", "str")
	})

	testRESP3(t, func(c *client) {
		c.Do("SADD", "s1", "aap", "noot", "mies")
		c.Do("SADD", "s2", "noot", "mies", "vuur")
		c.DoSorted("SDIFF", "s1")
		c.DoSorted("SDIFF", "s1", "s2")
		c.Do("SDIFFSTORE", "res", "s1", "s2")
		c.Do("SMEMBERS", "res")
	})
}

func TestSetSinter(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SADD", "s1", "aap", "noot", "mies")
		c.Do("SADD", "s2", "noot", "mies", "vuur")
		c.Do("SADD", "s3", "mies", "wim")
		c.DoSorted("SINTER", "s1")
		c.DoSorted("SINTER", "s1", "s2")
		c.DoSorted("SINTER", "s1", "s2", "s3")
		c.Do("SINTER", "nosuch")
		c.Do("SINTER", "s1", "nosuch", "s2", "nosuch", "s3")
		c.DoSorted("SINTER", "s1", "s1")

		c.Do("SINTERSTORE", "res", "s3", "nosuch", "s1")
		c.Do("SMEMBERS", "res")

		// failure cases
		c.Error("wrong number", "SINTER")
		c.Error("wrong number", "SINTERSTORE")
		c.Error("wrong number", "SINTERSTORE", "key")
		// Wrong type
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "SINTER", "s1", "str")
		c.Error("wrong kind", "SINTER", "nosuch", "str")
		c.Error("wrong kind", "SINTER", "str", "nosuch")
		c.Error("wrong kind", "SINTER", "str", "s1")
		c.Error("wrong kind", "SINTERSTORE", "res", "str", "s1")
		c.Error("wrong kind", "SINTERSTORE", "res", "s1", "str")
	})
}

func TestSetSintercard(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SADD", "s1", "aap", "noot", "mies")
		c.Do("SADD", "s2", "noot", "mies", "vuur")
		c.Do("SADD", "s3", "mies", "wim")
		c.Do("SINTERCARD", "1", "s1")
		c.Do("SINTERCARD", "2", "s1", "s2")
		c.Do("SINTERCARD", "3", "s1", "s2", "s3")
		c.Do("SINTERCARD", "1", "nosuch")
		c.Do("SINTERCARD", "5", "s1", "nosuch", "s2", "nosuch", "s3")
		c.Do("SINTERCARD", "2", "s1", "s1")

		c.Do("SINTERCARD", "1", "s1", "LIMIT", "1")
		c.Do("SINTERCARD", "2", "s1", "s2", "LIMIT", "0")
		c.Do("SINTERCARD", "2", "s1", "s2", "LIMIT", "1")
		c.Do("SINTERCARD", "2", "s1", "s2", "LIMIT", "2")
		c.Do("SINTERCARD", "2", "s1", "s2", "LIMIT", "3")

		// failure cases
		c.Error("wrong number", "SINTERCARD")
		c.Error("wrong number", "SINTERCARD", "0")
		c.Error("wrong number", "SINTERCARD", "")
		c.Error("wrong number", "SINTERCARD", "s1")
		c.Error("greater than 0", "SINTERCARD", "s1", "s2")
		c.Error("greater than 0", "SINTERCARD", "-2", "s1", "s2")
		c.Error("greater than 0", "SINTERCARD", "-1", "s1", "s2")
		c.Error("greater than 0", "SINTERCARD", "0", "s1", "s2")
		c.Error("syntax error", "SINTERCARD", "1", "s1", "s2")
		c.Error("can't be greater", "SINTERCARD", "3", "s1", "s2")
		// Wrong type
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "SINTERCARD", "2", "s1", "str")
		c.Error("wrong kind", "SINTERCARD", "2", "nosuch", "str")
		c.Error("wrong kind", "SINTERCARD", "2", "str", "nosuch")
		c.Error("wrong kind", "SINTERCARD", "2", "str", "s1")
		c.Error("can't be negative", "SINTERCARD", "2", "s1", "s2", "LIMIT", "-1")
	})
}

func TestSetSunion(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SUNION", "s1", "aap", "noot", "mies")
		c.Do("SUNION", "s2", "noot", "mies", "vuur")
		c.Do("SUNION", "s3", "mies", "wim")
		c.Do("SUNION", "s1")
		c.Do("SUNION", "s1", "s2")
		c.Do("SUNION", "s1", "s2", "s3")
		c.Do("SUNION", "nosuch")
		c.Do("SUNION", "s1", "nosuch", "s2", "nosuch", "s3")
		c.Do("SUNION", "s1", "s1")

		c.Do("SUNIONSTORE", "res", "s3", "nosuch", "s1")
		c.Do("SMEMBERS", "res")

		// failure cases
		c.Error("wrong number", "SUNION")
		c.Error("wrong number", "SUNIONSTORE")
		c.Error("wrong number", "SUNIONSTORE", "key")
		// Wrong type
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "SUNION", "s1", "str")
		c.Error("wrong kind", "SUNION", "nosuch", "str")
		c.Error("wrong kind", "SUNION", "str", "s1")
		c.Error("wrong kind", "SUNIONSTORE", "res", "str", "s1")
		c.Error("wrong kind", "SUNIONSTORE", "res", "s1", "str")
	})
}

func TestSscan(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		// No set yet
		c.Do("SSCAN", "set", "0")

		c.Do("SADD", "set", "key1")
		c.Do("SSCAN", "set", "0")
		c.Do("SSCAN", "set", "0", "COUNT", "12")
		c.Do("SSCAN", "set", "0", "cOuNt", "12")

		c.Do("SADD", "set", "anotherkey")
		c.Do("SSCAN", "set", "0", "MATCH", "anoth*")
		c.Do("SSCAN", "set", "0", "MATCH", "anoth*", "COUNT", "100")
		c.Do("SSCAN", "set", "0", "COUNT", "100", "MATCH", "anoth*")
		// c.DoLoosely("SSCAN", "set", "0", "COUNT", "1") // cursor differs // unstable test
		c.DoLoosely("SSCAN", "set", "0", "COUNT", "2") // cursor differs

		// Can't really test multiple keys.
		// c.Do("SET", "key2", "value2")
		// c.Do("SCAN", "0")

		// Error cases
		c.Error("wrong number", "SSCAN")
		c.Error("wrong number", "SSCAN", "noint")
		c.Error("not an integer", "SSCAN", "set", "0", "COUNT", "noint")
		c.Error("syntax error", "SSCAN", "set", "0", "COUNT", "0")
		c.Error("syntax error", "SSCAN", "set", "0", "COUNT")
		c.Error("syntax error", "SSCAN", "set", "0", "MATCH")
		c.Error("syntax error", "SSCAN", "set", "0", "garbage")
		c.Error("syntax error", "SSCAN", "set", "0", "COUNT", "12", "MATCH", "foo", "garbage")
		c.Do("SET", "str", "1")
		c.Error("wrong kind", "SSCAN", "str", "0")
	})
}

func TestSetNoAuth(t *testing.T) {
	skip(t)
	testAuth(t,
		"supersecret",
		func(c *client) {
			c.Error("Authentication required", "SET", "foo", "bar")
			c.Do("AUTH", "supersecret")
			c.Do("SET", "foo", "bar")
		},
	)
}
