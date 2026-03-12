package main

import (
	"sync"
	"testing"
)

func TestSubscribe(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Error("wrong number", "SUBSCRIBE")

		c.Do("SUBSCRIBE", "foo")
		c.Do("UNSUBSCRIBE")

		c.Do("SUBSCRIBE", "foo")
		c.Do("UNSUBSCRIBE", "foo")

		c.Do("SUBSCRIBE", "foo", "bar")
		c.Receive()
		c.Do("UNSUBSCRIBE", "foo", "bar")
		c.Receive()

		c.Do("SUBSCRIBE", "-1")
		c.Do("UNSUBSCRIBE", "-1")

		c.Do("UNSUBSCRIBE")
	})
}

func TestPsubscribe(t *testing.T) {
	skip(t)
	testRaw2(t, func(c1, c2 *client) {
		c1.Error("wrong number", "PSUBSCRIBE")

		c1.Do("PSUBSCRIBE", "foo")
		c2.Do("PUBLISH", "foo", "hi")
		c1.Receive()
		c1.Do("PUNSUBSCRIBE")

		c1.Do("PSUBSCRIBE", "foo")
		c2.Do("PUBLISH", "foo", "hi2")
		c1.Receive()
		c1.Do("PUNSUBSCRIBE", "foo")

		c1.Do("PSUBSCRIBE", "foo", "bar")
		c1.Receive()
		c2.Do("PUBLISH", "foo", "hi3")
		c1.Receive()
		c1.Do("PUNSUBSCRIBE", "foo", "bar")
		c1.Receive()

		c1.Do("PSUBSCRIBE", "f?o")
		c2.Do("PUBLISH", "foo", "hi4")
		c1.Receive()
		c1.Do("PUNSUBSCRIBE", "f?o")

		c1.Do("PSUBSCRIBE", "f*o")
		c2.Do("PUBLISH", "foo", "hi5")
		c1.Receive()
		c1.Do("PUNSUBSCRIBE", "f*o")

		c1.Do("PSUBSCRIBE", "f[oO]o")
		c2.Do("PUBLISH", "foo", "hi6")
		c1.Receive()
		c1.Do("PUNSUBSCRIBE", "f[oO]o")

		c1.Do("PSUBSCRIBE", `f\?o`)
		c2.Do("PUBLISH", "f?o", "hi7")
		c1.Receive()
		c1.Do("PUNSUBSCRIBE", `f\?o`)

		c1.Do("PSUBSCRIBE", `f\*o`)
		c2.Do("PUBLISH", "f*o", "hi8")
		c1.Receive()
		c1.Do("PUNSUBSCRIBE", `f\*o`)

		c1.Do("PSUBSCRIBE", "f\\[oO]o")
		c2.Do("PUBLISH", "f[oO]o", "hi9")
		c1.Receive()
		c1.Do("PUNSUBSCRIBE", "f\\[oO]o")

		c1.Do("PSUBSCRIBE", `f\\oo`)
		c2.Do("PUBLISH", `f\\oo`, "hi10")
		c1.Do("PUNSUBSCRIBE", `f\\oo`)

		c1.Do("PSUBSCRIBE", "-1")
		c2.Do("PUBLISH", "foo", "hi11")
		c1.Do("PUNSUBSCRIBE", "-1")
	})

	testRaw2(t, func(c1, c2 *client) {
		c1.Do("PSUBSCRIBE", "news*")
		c2.Do("PUBLISH", "news", "fire!")
		c1.Receive()
	})

	testRaw2(t, func(c1, c2 *client) {
		c1.Do("PSUBSCRIBE", "news") // no pattern
		c2.Do("PUBLISH", "news", "fire!")
		c1.Receive()
	})

	testRaw(t, func(c *client) {
		c.Do("PUNSUBSCRIBE")
		c.Do("PUNSUBSCRIBE")
	})
}

func TestPublish(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Error("wrong number", "PUBLISH")
		c.Error("wrong number", "PUBLISH", "foo")
		c.Do("PUBLISH", "foo", "bar")
		c.Error("wrong number", "PUBLISH", "foo", "bar", "deadbeef")
		c.Do("PUBLISH", "-1", "-2")
	})
}

func TestPubSub(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Error("wrong number", "PUBSUB")
		c.Error("subcommand", "PUBSUB", "FOO")

		c.Do("PUBSUB", "CHANNELS")
		c.Do("PUBSUB", "CHANNELS", "foo")
		c.Error("wrong number", "PUBSUB", "CHANNELS", "foo", "bar")
		c.Do("PUBSUB", "CHANNELS", "f?o")
		c.Do("PUBSUB", "CHANNELS", "f*o")
		c.Do("PUBSUB", "CHANNELS", "f[oO]o")
		c.Do("PUBSUB", "CHANNELS", "f\\?o")
		c.Do("PUBSUB", "CHANNELS", "f\\*o")
		c.Do("PUBSUB", "CHANNELS", "f\\[oO]o")
		c.Do("PUBSUB", "CHANNELS", "f\\\\oo")
		c.Do("PUBSUB", "CHANNELS", "-1")

		c.Do("PUBSUB", "NUMSUB")
		c.Do("PUBSUB", "NUMSUB", "foo")
		c.Do("PUBSUB", "NUMSUB", "foo", "bar")
		c.Do("PUBSUB", "NUMSUB", "-1")

		c.Do("PUBSUB", "NUMPAT")
		c.Error("wrong number", "PUBSUB", "NUMPAT", "foo")
	})
}

func TestPubsubFull(t *testing.T) {
	skip(t)
	testRaw2(t, func(c1, c2 *client) {
		c1.Do("SUBSCRIBE", "news", "sport")
		c1.Receive()
		c2.Do("PUBLISH", "news", "revolution!")
		c2.Do("PUBLISH", "news", "alien invasion!")
		c2.Do("PUBLISH", "sport", "lady biked too fast")
		c2.Do("PUBLISH", "gossip", "man bites dog")
		c1.Receive()
		c1.Receive()
		c1.Receive()
		c1.Do("UNSUBSCRIBE", "news", "sport")
		c1.Receive()
	})

	testRESP3Pair(t, func(c1, c2 *client) {
		c1.Do("SUBSCRIBE", "news", "sport")
		c1.Receive()
		c2.Do("PUBLISH", "news", "fire!")
		c1.Receive()
		c1.Do("UNSUBSCRIBE", "news", "sport")
		c1.Receive()
	})
}

func TestPubsubMulti(t *testing.T) {
	skip(t)
	var wg1 sync.WaitGroup
	wg1.Add(2)
	testMulti(t,
		func(c *client) {
			c.Do("SUBSCRIBE", "news", "sport")
			c.Receive()
			wg1.Done()
			c.Receive()
			c.Receive()
			c.Receive()
			c.Do("UNSUBSCRIBE", "news", "sport")
			c.Receive()
		},
		func(c *client) {
			c.Do("SUBSCRIBE", "sport")
			wg1.Done()
			c.Receive()
			c.Do("UNSUBSCRIBE", "sport")
		},
		func(c *client) {
			wg1.Wait()
			c.Do("PUBLISH", "news", "revolution!")
			c.Do("PUBLISH", "news", "alien invasion!")
			c.Do("PUBLISH", "sport", "lady biked too fast")
		},
	)
}

func TestPubsubSelect(t *testing.T) {
	skip(t)
	testRaw2(t, func(c1, c2 *client) {
		c1.Do("SUBSCRIBE", "news", "sport")
		c1.Receive()
		c2.Do("SELECT", "3")
		c2.Do("PUBLISH", "news", "revolution!")
		c1.Receive()
	})
}

func TestPubsubMode(t *testing.T) {
	skip(t)
	// most commands aren't allowed in publish mode
	t.Run("basic", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("SUBSCRIBE", "news", "sport")
			c.Receive()
			c.Do("PING")
			c.Do("PING", "foo")
			c.Error("are allowed", "ECHO", "foo")
			c.Error("are allowed", "HGET", "foo", "bar")
			c.Error("are allowed", "SET", "foo", "bar")
			c.Do("QUIT")
		})
	})

	t.Run("tx", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("SUBSCRIBE", "news")
			// failWith(e, "PING"),
			// failWith(e, "PSUBSCRIBE"),
			// failWith(e, "PUNSUBSCRIBE"),
			// failWith(e, "QUIT"),
			// failWith(e, "SUBSCRIBE"),
			// failWith(e, "UNSUBSCRIBE"),

			c.Error("are allowed", "APPEND", "foo", "foo")
			c.Error("are allowed", "AUTH", "foo")
			c.Error("are allowed", "BITCOUNT", "foo")
			c.Error("are allowed", "BITOP", "OR", "foo", "bar")
			c.Error("are allowed", "BITPOS", "foo", "0")
			c.Error("are allowed", "BLPOP", "key", "1")
			c.Error("are allowed", "BRPOP", "key", "1")
			c.Error("are allowed", "BRPOPLPUSH", "foo", "bar", "1")
			c.Error("are allowed", "DBSIZE")
			c.Error("are allowed", "DECR", "foo")
			c.Error("are allowed", "DECRBY", "foo", "3")
			c.Error("are allowed", "DEL", "foo")
			c.Error("are allowed", "DISCARD")
			c.Error("are allowed", "ECHO", "foo")
			c.Error("are allowed", "EVAL", "foo", "{}")
			c.Error("are allowed", "EVALSHA", "foo", "{}")
			c.Error("are allowed", "EXEC")
			c.Error("are allowed", "EXISTS", "foo")
			c.Error("are allowed", "EXPIRE", "foo", "12")
			c.Error("are allowed", "EXPIREAT", "foo", "12")
			c.Error("are allowed", "FLUSHALL")
			c.Error("are allowed", "FLUSHDB")
			c.Error("are allowed", "GET", "foo")
			c.Error("are allowed", "GETEX", "foo")
			c.Error("are allowed", "GETBIT", "foo", "12")
			c.Error("are allowed", "GETRANGE", "foo", "12", "12")
			c.Error("are allowed", "GETSET", "foo", "bar")
			c.Error("are allowed", "HDEL", "foo", "bar")
			c.Error("are allowed", "HEXISTS", "foo", "bar")
			c.Error("are allowed", "HGET", "foo", "bar")
			c.Error("are allowed", "HGETALL", "foo")
			c.Error("are allowed", "HINCRBY", "foo", "bar", "12")
			c.Error("are allowed", "HINCRBYFLOAT", "foo", "bar", "12.34")
			c.Error("are allowed", "HKEYS", "foo")
			c.Error("are allowed", "HLEN", "foo")
			c.Error("are allowed", "HMGET", "foo", "bar")
			c.Error("are allowed", "HMSET", "foo", "bar", "baz")
			c.Error("are allowed", "HSCAN", "foo", "0")
			c.Error("are allowed", "HSET", "foo", "bar", "baz")
			c.Error("are allowed", "HSETNX", "foo", "bar", "baz")
			c.Error("are allowed", "HVALS", "foo")
			c.Error("are allowed", "INCR", "foo")
			c.Error("are allowed", "INCRBY", "foo", "12")
			c.Error("are allowed", "INCRBYFLOAT", "foo", "12.34")
			c.Error("are allowed", "KEYS", "*")
			c.Error("are allowed", "LINDEX", "foo", "0")
			c.Error("are allowed", "LINSERT", "foo", "after", "bar", "0")
			c.Error("are allowed", "LLEN", "foo")
			c.Error("are allowed", "LPOP", "foo")
			c.Error("are allowed", "LPUSH", "foo", "bar")
			c.Error("are allowed", "LPUSHX", "foo", "bar")
			c.Error("are allowed", "LRANGE", "foo", "1", "1")
			c.Error("are allowed", "LREM", "foo", "0", "bar")
			c.Error("are allowed", "LSET", "foo", "0", "bar")
			c.Error("are allowed", "LTRIM", "foo", "0", "0")
			c.Error("are allowed", "MGET", "foo", "bar")
			c.Error("are allowed", "MOVE", "foo", "bar")
			c.Error("are allowed", "MSET", "foo", "bar")
			c.Error("are allowed", "MSETNX", "foo", "bar")
			c.Error("are allowed", "MULTI")
			c.Error("are allowed", "PERSIST", "foo")
			c.Error("are allowed", "PEXPIRE", "foo", "12")
			c.Error("are allowed", "PEXPIREAT", "foo", "12")
			c.Error("are allowed", "PSETEX", "foo", "12", "bar")
			c.Error("are allowed", "PTTL", "foo")
			c.Error("are allowed", "PUBLISH", "foo", "bar")
			c.Error("are allowed", "PUBSUB", "CHANNELS")
			c.Error("are allowed", "RANDOMKEY")
			c.Error("are allowed", "RENAME", "foo", "bar")
			c.Error("are allowed", "RENAMENX", "foo", "bar")
			c.Error("are allowed", "RPOP", "foo")
			c.Error("are allowed", "RPOPLPUSH", "foo", "bar")
			c.Error("are allowed", "RPUSH", "foo", "bar")
			c.Error("are allowed", "RPUSHX", "foo", "bar")
			c.Error("are allowed", "SADD", "foo", "bar")
			c.Error("are allowed", "SCAN", "0")
			c.Error("are allowed", "SCARD", "foo")
			c.Error("are allowed", "SCRIPT", "FLUSH")
			c.Error("are allowed", "SDIFF", "foo")
			c.Error("are allowed", "SDIFFSTORE", "foo", "bar")
			c.Error("are allowed", "SELECT", "12")
			c.Error("are allowed", "SET", "foo", "bar")
			c.Error("are allowed", "SETBIT", "foo", "0", "1")
			c.Error("are allowed", "SETEX", "foo", "12", "bar")
			c.Error("are allowed", "SETNX", "foo", "bar")
			c.Error("are allowed", "SETRANGE", "foo", "0", "bar")
			c.Error("are allowed", "SINTER", "foo", "bar")
			c.Error("are allowed", "SINTERSTORE", "foo", "bar", "baz")
			c.Error("are allowed", "SISMEMBER", "foo", "bar")
			c.Error("are allowed", "SMEMBERS", "foo")
			c.Error("are allowed", "SMOVE", "foo", "bar", "baz")
			c.Error("are allowed", "SPOP", "foo")
			c.Error("are allowed", "SRANDMEMBER", "foo")
			c.Error("are allowed", "SREM", "foo", "bar", "baz")
			c.Error("are allowed", "SSCAN", "foo", "0")
			c.Error("are allowed", "STRLEN", "foo")
			c.Error("are allowed", "SUNION", "foo", "bar")
			c.Error("are allowed", "SUNIONSTORE", "foo", "bar", "baz")
			c.Error("are allowed", "TIME")
			c.Error("are allowed", "TTL", "foo")
			c.Error("are allowed", "TYPE", "foo")
			c.Error("are allowed", "UNWATCH")
			c.Error("are allowed", "WATCH", "foo")
			c.Error("are allowed", "ZADD", "foo", "INCR", "1", "bar")
			c.Error("are allowed", "ZCARD", "foo")
			c.Error("are allowed", "ZCOUNT", "foo", "0", "1")
			c.Error("are allowed", "ZINCRBY", "foo", "bar", "12")
			c.Error("are allowed", "ZINTERSTORE", "foo", "1", "bar")
			c.Error("are allowed", "ZLEXCOUNT", "foo", "-", "+")
			c.Error("are allowed", "ZRANGE", "foo", "0", "-1")
			c.Error("are allowed", "ZRANGEBYLEX", "foo", "-", "+")
			c.Error("are allowed", "ZRANGEBYSCORE", "foo", "0", "1")
			c.Error("are allowed", "ZRANK", "foo", "bar")
			c.Error("are allowed", "ZREM", "foo", "bar")
			c.Error("are allowed", "ZREMRANGEBYLEX", "foo", "-", "+")
			c.Error("are allowed", "ZREMRANGEBYRANK", "foo", "0", "1")
			c.Error("are allowed", "ZREMRANGEBYSCORE", "foo", "0", "1")
			c.Error("are allowed", "ZREVRANGE", "foo", "0", "-1")
			c.Error("are allowed", "ZREVRANGEBYLEX", "foo", "+", "-")
			c.Error("are allowed", "ZREVRANGEBYSCORE", "foo", "0", "1")
			c.Error("are allowed", "ZREVRANK", "foo", "bar")
			c.Error("are allowed", "ZSCAN", "foo", "0")
			c.Error("are allowed", "ZSCORE", "foo", "bar")
			c.Error("are allowed", "ZUNIONSTORE", "foo", "1", "bar")
		})
	})
}

func TestSubscriptions(t *testing.T) {
	skip(t)
	testRaw2(t, func(c1, c2 *client) {
		c1.Do("SUBSCRIBE", "foo", "bar", "foo")
		c2.Do("PUBSUB", "NUMSUB")
		c1.Do("UNSUBSCRIBE", "bar", "bar", "bar")
		c2.Do("PUBSUB", "NUMSUB")
	})
}

func TestPubsubUnsub(t *testing.T) {
	skip(t)
	testRaw2(t, func(c1, c2 *client) {
		c1.Do("SUBSCRIBE", "news", "sport")
		c1.Receive()
		c2.DoSorted("PUBSUB", "CHANNELS")
		c1.Do("QUIT")
		c2.DoSorted("PUBSUB", "CHANNELS")
	})
}

func TestPubsubTx(t *testing.T) {
	skip(t)
	// publish is in a tx
	testRaw2(t, func(c1, c2 *client) {
		c1.Do("SUBSCRIBE", "foo")
		c2.Do("MULTI")
		c2.Do("PUBSUB", "CHANNELS")
		c2.Do("PUBLISH", "foo", "hello one")
		c2.Error("wrong number", "GET")
		c2.Do("PUBLISH", "foo", "hello two")
		c2.Error("discarded", "EXEC")

		c2.Do("PUBLISH", "foo", "post tx")
		c1.Receive()
	})

	// SUBSCRIBE is in a tx
	testRaw2(t, func(c1, c2 *client) {
		c1.Do("MULTI")
		c1.Do("SUBSCRIBE", "foo")
		c2.Do("PUBSUB", "CHANNELS")
		c1.Do("EXEC")
		c2.Do("PUBSUB", "CHANNELS")

		c1.Error("are allowed", "MULTI") // we're in SUBSCRIBE mode
	})

	// DISCARDing a tx prevents from entering publish mode
	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("SUBSCRIBE", "foo")
		c.Do("DISCARD")
		c.Do("PUBSUB", "CHANNELS")
	})

	// UNSUBSCRIBE is in a tx
	testRaw2(t, func(c1, c2 *client) {
		c1.Do("MULTI")
		c1.Do("SUBSCRIBE", "foo")
		c1.Do("UNSUBSCRIBE", "foo")
		c2.Do("PUBSUB", "CHANNELS")
		c1.Do("EXEC")
		c2.Do("PUBSUB", "CHANNELS")
		c1.Do("PUBSUB", "CHANNELS")
	})

	// PSUBSCRIBE is in a tx
	testRaw2(t, func(c1, c2 *client) {
		c1.Do("MULTI")
		c1.Do("PSUBSCRIBE", "foo")
		c2.Do("PUBSUB", "NUMPAT")
		c1.Do("EXEC")
		c2.Do("PUBSUB", "NUMPAT")

		c1.Error("are allowed", "MULTI") // we're in SUBSCRIBE mode
	})

	// PUNSUBSCRIBE is in a tx
	testRaw2(t, func(c1, c2 *client) {
		c1.Do("MULTI")
		c1.Do("PSUBSCRIBE", "foo")
		c1.Do("PUNSUBSCRIBE", "foo")
		c2.Do("PUBSUB", "NUMPAT")
		c1.Do("EXEC")
		c2.Do("PUBSUB", "NUMPAT")
		c1.Do("PUBSUB", "NUMPAT")
	})
}
