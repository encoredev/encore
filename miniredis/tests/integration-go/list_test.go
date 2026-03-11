package main

import (
	"sync"
	"testing"
	"time"
)

func TestLPushLpop(t *testing.T) {
	skip(t)
	t.Run("without count", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("LPUSH", "l", "aap", "noot", "mies")
			c.Do("TYPE", "l")
			c.Do("LPUSH", "l", "more", "keys")
			c.Do("LRANGE", "l", "0", "-1")
			c.Do("LRANGE", "l", "0", "6")
			c.Do("LRANGE", "l", "2", "6")
			c.Do("LRANGE", "l", "-100", "-100")
			c.Do("LRANGE", "nosuch", "2", "6")
			c.Do("LPOP", "l")
			c.Do("LPOP", "l")
			c.Do("LPOP", "l")
			c.Do("LPOP", "l")
			c.Do("LPOP", "l")
			c.Do("LPOP", "l")
			c.Do("EXISTS", "l")
			c.Do("LPOP", "nosuch")

			// failure cases
			c.Error("wrong number", "LPUSH")
			c.Error("wrong number", "LPUSH", "l")
			c.Do("SET", "str", "I am a string")
			c.Error("wrong kind", "LPUSH", "str", "noot", "mies")
			c.Error("wrong number", "LRANGE")
			c.Error("wrong number", "LRANGE", "key")
			c.Error("wrong number", "LRANGE", "key", "2")
			c.Error("wrong number", "LRANGE", "key", "2", "6", "toomany")
			c.Error("not an integer", "LRANGE", "key", "noint", "6")
			c.Error("not an integer", "LRANGE", "key", "2", "noint")
			c.Error("wrong number", "LPOP")
		})
	})

	t.Run("with count", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("LPUSH", "l", "aap", "noot", "mies")
			c.Do("LPOP", "l", "0")
			c.Do("LPOP", "l", "2")
			c.Do("LPOP", "l", "2")
			c.Error("out of range", "LPOP", "l", "-42")
			c.Do("LPOP", "nosuch", "2")

			c.Error("wrong number", "LPOP", "nosuch", "2", "foobar")
		})
	})

	t.Run("resp3", func(t *testing.T) {
		testRESP3(t, func(c *client) {
			c.Do("LPUSH", "l", "aap", "noot", "mies")
			c.Do("LPOP", "l")
			c.Do("LPOP", "l", "9")
			c.Do("LPOP", "l")
			c.Do("LPOP", "l", "9")
			c.Do("LPOP", "nosuch")
			c.Do("LPOP", "nosuch", "9")
		})
	})
}

func TestLPushx(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("LPUSHX", "l", "aap")
		c.Do("EXISTS", "l")
		c.Do("LRANGE", "l", "0", "-1")
		c.Do("LPUSH", "l", "noot")
		c.Do("LPUSHX", "l", "mies")
		c.Do("EXISTS", "l")
		c.Do("LRANGE", "l", "0", "-1")
		c.Do("LPUSHX", "l", "even", "more", "arguments")

		// failure cases
		c.Error("wrong number", "LPUSHX")
		c.Error("wrong number", "LPUSHX", "l")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "LPUSHX", "str", "mies")
	})
}

func TestRPushRPop(t *testing.T) {
	skip(t)
	t.Run("without count", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("RPUSH", "l", "aap", "noot", "mies")
			c.Do("TYPE", "l")
			c.Do("RPUSH", "l", "more", "keys")
			c.Do("LRANGE", "l", "0", "-1")
			c.Do("LRANGE", "l", "0", "6")
			c.Do("LRANGE", "l", "2", "6")
			c.Do("RPOP", "l")
			c.Do("RPOP", "l")
			c.Do("RPOP", "l")
			c.Do("RPOP", "l")
			c.Do("RPOP", "l")
			c.Do("RPOP", "l")
			c.Do("EXISTS", "l")
			c.Do("RPOP", "nosuch")

			// failure cases
			c.Error("wrong number", "RPUSH")
			c.Error("wrong number", "RPUSH", "l")
			c.Do("SET", "str", "I am a string")
			c.Error("wrong kind", "RPUSH", "str", "noot", "mies")
			c.Error("wrong number", "RPOP")
		})
	})

	t.Run("with count", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("RPUSH", "l", "aap", "noot", "mies")
			c.Do("RPOP", "l", "0")
			c.Do("RPOP", "l", "2")
			c.Do("RPOP", "l", "99")
			c.Do("RPOP", "l", "99")
			c.Do("RPOP", "nosuch", "99")
		})
	})

	t.Run("resp3", func(t *testing.T) {
		testRESP3(t, func(c *client) {
			c.Do("RPUSH", "l", "aap", "noot", "mies")
			c.Do("RPOP", "l")
			c.Do("RPOP", "l", "9")
			c.Do("RPOP", "l")
			c.Do("RPOP", "l", "9")
		})
	})
}

func TestLinxed(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("RPUSH", "l", "aap", "noot", "mies")
		c.Do("LINDEX", "l", "0")
		c.Do("LINDEX", "l", "1")
		c.Do("LINDEX", "l", "2")
		c.Do("LINDEX", "l", "3")
		c.Do("LINDEX", "l", "4")
		c.Do("LINDEX", "l", "44444")
		c.Error("not an integer", "LINDEX", "l", "-0")
		c.Do("LINDEX", "l", "-1")
		c.Do("LINDEX", "l", "-2")
		c.Do("LINDEX", "l", "-3")
		c.Do("LINDEX", "l", "-4")
		c.Do("LINDEX", "l", "-4000")

		// failure cases
		c.Error("wrong number", "LINDEX")
		c.Error("wrong number", "LINDEX", "l")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "LINDEX", "str", "1")
		c.Error("not an integer", "LINDEX", "l", "noint")
		c.Error("wrong number", "LINDEX", "l", "1", "too many")
	})
}

func TestLpos(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("RPUSH", "l", "aap", "noot", "aap", "mies", "aap", "vuur", "aap", "aap")
		c.Do("LPOS", "l", "app")
		c.Do("LPOS", "l", "noot")
		c.Do("LPOS", "l", "mies")
		c.Do("LPOS", "l", "vuur")
		c.Do("LPOS", "l", "wim")
		c.Do("LPOS", "l", "app", "RANK", "1")
		c.Do("LPOS", "l", "app", "RANK", "4")
		c.Do("LPOS", "l", "app", "RANK", "5")
		c.Do("LPOS", "l", "app", "RANK", "6")
		c.Do("LPOS", "l", "app", "RANK", "-1")
		c.Do("LPOS", "l", "app", "RANK", "-4")
		c.Do("LPOS", "l", "app", "RANK", "-5")
		c.Do("LPOS", "l", "app", "RANK", "-6")
		c.Do("LPOS", "l", "wim", "COUNT", "1")
		c.Do("LPOS", "l", "aap", "COUNT", "1")
		c.Do("LPOS", "l", "aap", "COUNT", "3")
		c.Do("LPOS", "l", "aap", "COUNT", "5")
		c.Do("LPOS", "l", "aap", "COUNT", "100")
		c.Do("LPOS", "l", "aap", "COUNT", "0")
		c.Do("LPOS", "l", "aap", "RANK", "3", "COUNT", "2")
		c.Do("LPOS", "l", "aap", "RANK", "3", "COUNT", "3")
		c.Do("LPOS", "l", "aap", "RANK", "5", "COUNT", "100")
		c.Do("LPOS", "l", "aap", "RANK", "-3", "COUNT", "2")
		c.Do("LPOS", "l", "aap", "RANK", "-3", "COUNT", "3")
		c.Do("LPOS", "l", "aap", "RANK", "-5", "COUNT", "100")
		c.Do("LPOS", "l", "aap", "RANK", "4", "MAXLEN", "6")
		c.Do("LPOS", "l", "aap", "RANK", "4", "MAXLEN", "7")
		c.Do("LPOS", "l", "aap", "RANK", "-4", "MAXLEN", "5")
		c.Do("LPOS", "l", "aap", "RANK", "-4", "MAXLEN", "6")
		c.Do("LPOS", "l", "aap", "COUNT", "0", "MAXLEN", "1")
		c.Do("LPOS", "l", "aap", "COUNT", "0", "MAXLEN", "4")
		c.Do("LPOS", "l", "aap", "COUNT", "0", "MAXLEN", "7")
		c.Do("LPOS", "l", "aap", "COUNT", "0", "MAXLEN", "8")
		c.Do("LPOS", "l", "aap", "COUNT", "2", "MAXLEN", "0")
		c.Do("LPOS", "l", "aap", "COUNT", "1", "MAXLEN", "0")
		c.Do("LPOS", "l", "aap", "RANK", "4", "COUNT", "2", "MAXLEN", "0")
		c.Do("LPOS", "l", "aap", "RANK", "4", "COUNT", "2", "MAXLEN", "7")
		c.Do("LPOS", "l", "aap", "RANK", "4", "COUNT", "2", "MAXLEN", "6")
		c.Do("LPOS", "l", "aap", "RANK", "-3", "COUNT", "2", "MAXLEN", "0")
		c.Do("LPOS", "l", "aap", "RANK", "-3", "COUNT", "2", "MAXLEN", "4")
		c.Do("LPOS", "l", "aap", "RANK", "-3", "COUNT", "2", "MAXLEN", "3")

		// failure cases
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "LPOS", "str", "aap")
		c.Error("wrong number", "LPOS", "l")
		c.Error("syntax error", "LPOS", "l", "aap", "RANK")
		c.Error("syntax error", "LPOS", "l", "aap", "RANK", "1", "COUNT")
		c.Error("syntax error", "LPOS", "l", "aap", "RANK", "1", "COUNT", "1", "MAXLEN")
		c.Error("syntax error", "LPOS", "l", "aap", "RANK", "1", "COUNT", "1", "MAXLEN", "1", "RANK")
		c.Error("syntax error", "LPOS", "l", "aap", "RANKS", "1")
		c.Error("syntax error", "LPOS", "l", "aap", "RANK", "1", "COUNTING", "1")
		c.Error("syntax error", "LPOS", "l", "aap", "RANK", "1", "MAXLENGTH", "1")
		c.Error("not an integer", "LPOS", "l", "aap", "RANK", "not_an_int")
		c.Error("can't be zero", "LPOS", "l", "aap", "RANK", "0")
		c.Error("can't be negative", "LPOS", "l", "aap", "COUNT", "-1")
		c.Error("can't be negative", "LPOS", "l", "aap", "COUNT", "not_an_int")
		c.Error("can't be negative", "LPOS", "l", "aap", "MAXLEN", "-1")
		c.Error("can't be negative", "LPOS", "l", "aap", "MAXLEN", "not_an_int")
		c.Error("can't be negative", "LPOS", "l", "aap", "MAXLEN", "-1", "RANK", "not_an_int", "COUNT", "-1")
		c.Error("not an integer", "LPOS", "l", "aap", "RANK", "not_an_int", "COUNT", "-1", "MAXLEN", "-1")
		c.Error("can't be negative", "LPOS", "l", "aap", "COUNT", "-1", "MAXLEN", "-1", "RANK", "not_an_int")
	})
}

func TestLlen(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("RPUSH", "l", "aap", "noot", "mies")
		c.Do("LLEN", "l")
		c.Do("LLEN", "nosuch")

		// failure cases
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "LLEN", "str")
		c.Error("wrong number", "LLEN")
		c.Error("wrong number", "LLEN", "l", "too many")
	})
}

func TestLtrim(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("RPUSH", "l", "aap", "noot", "mies")
		c.Do("LTRIM", "l", "0", "1")
		c.Do("LRANGE", "l", "0", "-1")
		c.Do("RPUSH", "l2", "aap", "noot", "mies", "vuur")
		c.Do("LTRIM", "l2", "-2", "-1")
		c.Do("LRANGE", "l2", "0", "-1")
		c.Do("RPUSH", "l3", "aap", "noot", "mies", "vuur")
		c.Do("LTRIM", "l3", "-2", "-1000")
		c.Do("LRANGE", "l3", "0", "-1")

		// remove the list
		c.Do("RPUSH", "l4", "aap")
		c.Do("LTRIM", "l4", "0", "-999")
		c.Do("EXISTS", "l4")

		// failure cases
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "LTRIM", "str", "0", "1")
		c.Error("wrong number", "LTRIM", "l", "0", "1", "toomany")
		c.Error("not an integer", "LTRIM", "l", "noint", "1")
		c.Error("not an integer", "LTRIM", "l", "0", "noint")
		c.Error("wrong number", "LTRIM", "l", "0")
		c.Error("wrong number", "LTRIM", "l")
		c.Error("wrong number", "LTRIM")
	})
}

func TestLrem(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("RPUSH", "l", "aap", "noot", "mies", "mies", "mies")
		c.Do("LREM", "l", "1", "mies")
		c.Do("LRANGE", "l", "0", "-1")
		c.Do("RPUSH", "l2", "aap", "noot", "mies", "mies", "mies")
		c.Do("LREM", "l2", "-2", "mies")
		c.Do("LRANGE", "l2", "0", "-1")
		c.Do("RPUSH", "l3", "aap", "noot", "mies", "mies", "mies")
		c.Do("LREM", "l3", "0", "mies")
		c.Do("LRANGE", "l3", "0", "-1")

		// remove the list
		c.Do("RPUSH", "l4", "aap")
		c.Do("LREM", "l4", "999", "aap")
		c.Do("EXISTS", "l4")

		// failure cases
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "LREM", "str", "0", "aap")
		c.Error("wrong number", "LREM", "l", "0", "aap", "toomany")
		c.Error("not an integer", "LREM", "l", "noint", "aap")
		c.Error("wrong number", "LREM", "l", "0")
		c.Error("wrong number", "LREM", "l")
		c.Error("wrong number", "LREM")
	})
}

func TestLset(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("RPUSH", "l", "aap", "noot", "mies", "mies", "mies")
		c.Do("LSET", "l", "1", "[cencored]")
		c.Do("LRANGE", "l", "0", "-1")
		c.Do("LSET", "l", "-1", "[cencored]")
		c.Do("LRANGE", "l", "0", "-1")
		c.Error("out of range", "LSET", "l", "1000", "new")
		c.Error("out of range", "LSET", "l", "-7000", "new")
		c.Error("no such key", "LSET", "nosuch", "1", "new")

		// failure cases
		c.Error("wrong number", "LSET")
		c.Error("wrong number", "LSET", "l")
		c.Error("wrong number", "LSET", "l", "0")
		c.Error("not an integer", "LSET", "l", "noint", "aap")
		c.Error("wrong number", "LSET", "l", "0", "aap", "toomany")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "LSET", "str", "0", "aap")
	})
}

func TestLinsert(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("RPUSH", "l", "aap", "noot", "mies", "mies", "mies!")
		c.Do("LINSERT", "l", "before", "aap", "1")
		c.Do("LINSERT", "l", "before", "noot", "2")
		c.Do("LINSERT", "l", "after", "mies!", "3")
		c.Do("LINSERT", "l", "after", "mies", "4")
		c.Do("LINSERT", "l", "after", "nosuch", "0")
		c.Do("LINSERT", "nosuch", "after", "nosuch", "0")
		c.Do("LRANGE", "l", "0", "-1")
		c.Do("LINSERT", "l", "AfTeR", "mies", "4")
		c.Do("LRANGE", "l", "0", "-1")

		// failure cases
		c.Error("wrong number", "LINSERT")
		c.Error("wrong number", "LINSERT", "l")
		c.Error("wrong number", "LINSERT", "l", "before")
		c.Error("wrong number", "LINSERT", "l", "before", "aap")
		c.Error("wrong number", "LINSERT", "l", "before", "aap", "too", "many")
		c.Error("syntax error", "LINSERT", "l", "What?", "aap", "noot")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "LINSERT", "str", "before", "aap", "noot")
	})
}

func TestRpoplpush(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("RPUSH", "l", "aap", "noot", "mies")
		c.Do("RPOPLPUSH", "l", "l2")
		c.Do("LRANGE", "l", "0", "-1")
		c.Do("LRANGE", "2l", "0", "-1")
		c.Do("RPOPLPUSH", "l", "l2")
		c.Do("RPOPLPUSH", "l", "l2")
		c.Do("RPOPLPUSH", "l", "l2") // now empty
		c.Do("EXISTS", "l")
		c.Do("LRANGE", "2l", "0", "-1")

		c.Do("RPUSH", "round", "aap", "noot", "mies")
		c.Do("RPOPLPUSH", "round", "round")
		c.Do("LRANGE", "round", "0", "-1")
		c.Do("RPOPLPUSH", "round", "round")
		c.Do("RPOPLPUSH", "round", "round")
		c.Do("RPOPLPUSH", "round", "round")
		c.Do("RPOPLPUSH", "round", "round")
		c.Do("LRANGE", "round", "0", "-1")

		// failure cases
		c.Do("RPUSH", "chk", "aap", "noot", "mies")
		c.Error("wrong number", "RPOPLPUSH")
		c.Error("wrong number", "RPOPLPUSH", "chk")
		c.Error("wrong number", "RPOPLPUSH", "chk", "too", "many")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "RPOPLPUSH", "chk", "str")
		c.Error("wrong kind", "RPOPLPUSH", "str", "chk")
		c.Do("LRANGE", "chk", "0", "-1")
	})
}

func TestRpushx(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("RPUSHX", "l", "aap")
		c.Do("EXISTS", "l")
		c.Do("RPUSH", "l", "noot", "mies")
		c.Do("RPUSHX", "l", "vuur")
		c.Do("EXISTS", "l")
		c.Do("LRANGE", "l", "0", "-1")
		c.Do("RPUSHX", "l", "more", "arguments")

		// failure cases
		c.Do("RPUSH", "chk", "noot", "mies")
		c.Error("wrong number", "RPUSHX")
		c.Error("wrong number", "RPUSHX", "chk")
		c.Do("LRANGE", "chk", "0", "-1")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "RPUSHX", "str", "value")
	})
}

func TestBrpop(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("LPUSH", "l", "one")
		c.Do("BRPOP", "l", "1")
		c.Do("BRPOP", "l", "0.1")
		c.Error("timeout is out of range", "BRPOP", "l", "inf")
		c.Do("EXISTS", "l")

		// transaction
		c.Do("MULTI")
		c.Do("BRPOP", "nosuch", "10")
		c.Do("EXEC")

		// failure cases
		c.Error("wrong number", "BRPOP")
		c.Error("wrong number", "BRPOP", "l")
		c.Error("not a float", "BRPOP", "l", "X")
		c.Error("not a float", "BRPOP", "l", "")
		c.Error("wrong number", "BRPOP", "1")
		c.Error("timeout is negative", "BRPOP", "key", "-1")
	})
}

func TestBrpopMulti(t *testing.T) {
	skip(t)
	testMulti(t,
		func(c *client) {
			c.Do("BRPOP", "key", "1")
			c.Do("BRPOP", "key", "1")
			c.Do("BRPOP", "key", "1")
			c.Do("BRPOP", "key", "1")
			c.Do("BRPOP", "key", "1") // will timeout
		},
		func(c *client) {
			c.Do("LPUSH", "key", "aap", "noot", "mies")
			time.Sleep(50 * time.Millisecond)
			c.Do("LPUSH", "key", "toon")
		},
	)
}

func TestBrpopTrans(t *testing.T) {
	skip(t)
	testMulti(t,
		func(c *client) {
			c.Do("BRPOP", "key", "1")
		},
		func(c *client) {
			c.Do("MULTI")
			c.Do("LPUSH", "key", "toon")
			c.Do("EXEC")
		},
	)
}

func TestBlpop(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("LPUSH", "l", "one")
		c.Do("BLPOP", "l", "1")
		c.Do("EXISTS", "l")

		// failure cases
		c.Error("wrong number", "BLPOP")
		c.Error("wrong number", "BLPOP", "l")
		c.Error("not a float", "BLPOP", "l", "X")
		c.Error("not a float", "BLPOP", "l", "")
		c.Error("wrong number", "BLPOP", "1")
		c.Error("timeout is negative", "BLPOP", "key", "-1")
	})

	testMulti(t,
		func(c *client) {
			c.Do("BLPOP", "key", "1")
			c.Do("BLPOP", "key", "1")
			c.Do("BLPOP", "key", "1")
			c.Do("BLPOP", "key", "1")
			c.Do("BLPOP", "key", "1") // will timeout
		},
		func(c *client) {
			c.Do("LPUSH", "key", "aap", "noot", "mies")
			time.Sleep(10 * time.Millisecond)
			c.Do("LPUSH", "key", "toon")
		},
	)
}

func TestBrpoplpush(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("LPUSH", "l", "one")
		c.Do("BRPOPLPUSH", "l", "l2", "0.1")
		c.Do("EXISTS", "l")
		c.Do("EXISTS", "l2")
		c.Do("LRANGE", "l", "0", "-1")
		c.Do("LRANGE", "l2", "0", "-1")

		// failure cases
		c.Error("wrong number", "BRPOPLPUSH")
		c.Error("wrong number", "BRPOPLPUSH", "l")
		c.Error("wrong number", "BRPOPLPUSH", "l", "x")
		c.Error("wrong number", "BRPOPLPUSH", "1")
		c.Error("timeout is negative", "BRPOPLPUSH", "from", "to", "-1")
		c.Error("out of range", "BRPOPLPUSH", "from", "to", "inf")
		c.Error("wrong number", "BRPOPLPUSH", "from", "to", "-1", "xxx")
	})

	wg := &sync.WaitGroup{}
	wg.Add(1)
	testMulti(t,
		func(c *client) {
			c.Do("BRPOPLPUSH", "from", "to", "1")
			c.Do("BRPOPLPUSH", "from", "to", "1")
			c.Do("BRPOPLPUSH", "from", "to", "1")
			c.Do("BRPOPLPUSH", "from", "to", "1")
			c.Do("BRPOPLPUSH", "from", "to", "1") // will timeout
			wg.Done()
		},
		func(c *client) {
			c.Do("LPUSH", "from", "aap", "noot", "mies")
			time.Sleep(20 * time.Millisecond)
			c.Do("LPUSH", "from", "toon")
			wg.Wait()
			c.Do("LRANGE", "from", "0", "-1")
			c.Do("LRANGE", "to", "0", "-1")
		},
	)
}

func TestLmove(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("RPUSH", "src", "LR", "LL", "RR", "RL")
		c.Do("LMOVE", "src", "dst", "LEFT", "RIGHT")
		c.Do("LRANGE", "src", "0", "-1")
		c.Do("LRANGE", "dst", "0", "-1")
		c.Do("LMOVE", "src", "dst", "RIGHT", "LEFT")
		c.Do("LMOVE", "src", "dst", "LEFT", "LEFT")
		c.Do("LMOVE", "src", "dst", "RIGHT", "RIGHT") // now empty
		c.Do("EXISTS", "src")
		c.Do("LRANGE", "dst", "0", "-1")

		// Cycle left to right
		c.Do("RPUSH", "round", "aap", "noot", "mies")
		c.Do("LMOVE", "round", "round", "LEFT", "RIGHT")
		c.Do("LRANGE", "round", "0", "-1")
		c.Do("LMOVE", "round", "round", "LEFT", "RIGHT")
		c.Do("LMOVE", "round", "round", "LEFT", "RIGHT")
		c.Do("LMOVE", "round", "round", "LEFT", "RIGHT")
		c.Do("LMOVE", "round", "round", "LEFT", "RIGHT")
		c.Do("LRANGE", "round", "0", "-1")
		// Cycle right to left
		c.Do("LMOVE", "round", "round", "RIGHT", "LEFT")
		c.Do("LRANGE", "round", "0", "-1")
		c.Do("LMOVE", "round", "round", "RIGHT", "LEFT")
		c.Do("LMOVE", "round", "round", "RIGHT", "LEFT")
		c.Do("LMOVE", "round", "round", "RIGHT", "LEFT")
		c.Do("LMOVE", "round", "round", "RIGHT", "LEFT")
		c.Do("LRANGE", "round", "0", "-1")
		// Cycle same side
		c.Do("LMOVE", "round", "round", "LEFT", "LEFT")
		c.Do("LRANGE", "round", "0", "-1")
		c.Do("LMOVE", "round", "round", "RIGHT", "RIGHT")
		c.Do("LRANGE", "round", "0", "-1")

		// failure cases
		c.Do("RPUSH", "chk", "aap", "noot", "mies")
		c.Error("wrong number", "LMOVE")
		c.Error("wrong number", "LMOVE", "chk")
		c.Error("wrong number", "LMOVE", "chk", "dst")
		c.Error("wrong number", "LMOVE", "chk", "dst", "chk")
		c.Error("wrong number", "LMOVE", "chk", "dst", "chk", "too", "many")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "LMOVE", "chk", "str", "LEFT", "LEFT")
		c.Error("wrong kind", "LMOVE", "str", "chk", "LEFT", "LEFT")
		c.Do("LRANGE", "chk", "0", "-1")
	})
}

func TestBlmove(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("RPUSH", "src", "LR", "LL", "RR", "RL")
		c.Do("BLMOVE", "src", "dst", "LEFT", "RIGHT", "0")
		c.Do("LRANGE", "src", "0", "-1")
		c.Do("LRANGE", "dst", "0", "-1")
		c.Do("BLMOVE", "src", "dst", "RIGHT", "LEFT", "0")
		c.Do("BLMOVE", "src", "dst", "LEFT", "LEFT", "0")
		c.Do("BLMOVE", "src", "dst", "RIGHT", "RIGHT", "0") // now empty
		c.Do("EXISTS", "src")
		c.Do("LRANGE", "dst", "0", "-1")

		// Cycle left to right
		c.Do("RPUSH", "round", "aap", "noot", "mies")
		c.Do("BLMOVE", "round", "round", "LEFT", "RIGHT", "0")
		c.Do("LRANGE", "round", "0", "-1")
		c.Do("BLMOVE", "round", "round", "LEFT", "RIGHT", "0")
		c.Do("BLMOVE", "round", "round", "LEFT", "RIGHT", "0")
		c.Do("BLMOVE", "round", "round", "LEFT", "RIGHT", "0")
		c.Do("BLMOVE", "round", "round", "LEFT", "RIGHT", "0")
		c.Do("LRANGE", "round", "0", "-1")
		// Cycle right to left
		c.Do("BLMOVE", "round", "round", "RIGHT", "LEFT", "0")
		c.Do("LRANGE", "round", "0", "-1")
		c.Do("BLMOVE", "round", "round", "RIGHT", "LEFT", "0")
		c.Do("BLMOVE", "round", "round", "RIGHT", "LEFT", "0")
		c.Do("BLMOVE", "round", "round", "RIGHT", "LEFT", "0")
		c.Do("BLMOVE", "round", "round", "RIGHT", "LEFT", "0")
		c.Do("LRANGE", "round", "0", "-1")
		// Cycle same side
		c.Do("BLMOVE", "round", "round", "LEFT", "LEFT", "0")
		c.Do("LRANGE", "round", "0", "-1")
		c.Do("BLMOVE", "round", "round", "RIGHT", "RIGHT", "0")
		c.Do("LRANGE", "round", "0", "-1")

		// TTL
		c.Do("LPUSH", "test", "1")
		c.Do("EXPIRE", "test", "1000")
		c.Do("TTL", "test")
		c.Do("BLMOVE", "test", "test", "LEFT", "LEFT", "1")
		c.Do("TTL", "test")

		// failure cases
		c.Do("RPUSH", "chk", "aap", "noot", "mies")
		c.Error("wrong number", "LMOVE")
		c.Error("wrong number", "LMOVE", "chk")
		c.Error("wrong number", "LMOVE", "chk", "dst")
		c.Error("wrong number", "LMOVE", "chk", "dst", "chk")
		c.Error("wrong number", "LMOVE", "chk", "dst", "chk", "too", "many")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "BLMOVE", "chk", "str", "LEFT", "LEFT", "0")
		c.Error("wrong kind", "BLMOVE", "str", "chk", "LEFT", "LEFT", "0")
		c.Do("LRANGE", "chk", "0", "-1")
	})

	wg := &sync.WaitGroup{}
	wg.Add(1)
	testMulti(t,
		func(c *client) {
			c.Do("BLMOVE", "from", "to", "RIGHT", "LEFT", "1")
			c.Do("BLMOVE", "from", "to", "RIGHT", "LEFT", "1")
			c.Do("BLMOVE", "from", "to", "RIGHT", "LEFT", "1")
			c.Do("BLMOVE", "from", "to", "RIGHT", "LEFT", "1")
			c.Do("BLMOVE", "from", "to", "RIGHT", "LEFT", "1") // will timeout
			wg.Done()
		},
		func(c *client) {
			c.Do("LPUSH", "from", "aap", "noot", "mies")
			time.Sleep(20 * time.Millisecond)
			c.Do("LPUSH", "from", "toon")
			wg.Wait()
			c.Do("LRANGE", "from", "0", "-1")
			c.Do("LRANGE", "to", "0", "-1")
		},
	)
}
