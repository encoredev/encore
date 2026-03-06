package main

import (
	"testing"
)

func TestSortedSet(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ZADD", "z", "1", "aap", "2", "noot", "3", "mies")
		c.Do("ZADD", "z", "1", "vuur", "4", "noot")
		c.Do("TYPE", "z")
		c.Do("EXISTS", "z")
		c.Do("ZCARD", "z")

		c.Do("ZRANK", "z", "aap")
		c.Do("ZRANK", "z", "noot")
		c.Do("ZRANK", "z", "mies")
		c.Do("ZRANK", "z", "vuur")
		c.Do("ZRANK", "z", "nosuch")
		c.Do("ZRANK", "z", "vuur", "WITHSCORE")
		c.Do("ZRANK", "nosuch", "nosuch")
		c.Do("ZRANK", "z", "nosuch", "WITHSCORE")
		c.Do("ZRANK", "nosuch", "nosuch", "WITHSCORE")
		c.Do("ZREVRANK", "z", "aap")
		c.Do("ZREVRANK", "z", "noot")
		c.Do("ZREVRANK", "z", "mies")
		c.Do("ZREVRANK", "z", "vuur")
		c.Do("ZREVRANK", "z", "nosuch")
		c.Do("ZREVRANK", "nosuch", "nosuch")
		c.Do("ZREVRANK", "z", "noot", "WITHSCORE")
		c.Do("ZREVRANK", "nosuch", "nosuch", "WITHSCORE")

		c.Do("ZADD", "zi", "inf", "aap", "-inf", "noot", "+inf", "mies")
		c.Do("ZRANK", "zi", "noot")

		// Double key
		c.Do("ZADD", "zz", "1", "aap", "2", "aap")
		c.Do("ZCARD", "zz")

		c.Do("ZPOPMAX", "zz", "2")
		c.Do("ZPOPMAX", "zz")
		c.Error("out of range", "ZPOPMAX", "zz", "-100")
		c.Do("ZPOPMAX", "nosuch", "1")
		c.Do("ZPOPMAX", "zz", "0")
		c.Do("ZPOPMAX", "zz", "100")

		c.Do("ZPOPMIN", "zz", "2")
		c.Do("ZPOPMIN", "zz")
		c.Error("out of range", "ZPOPMIN", "zz", "-100")
		c.Do("ZPOPMIN", "nosuch", "1")
		c.Do("ZPOPMIN", "zz", "0")
		c.Do("ZPOPMIN", "zz", "100")

		// failure cases
		c.Do("SET", "str", "I am a string")
		c.Error("wrong number", "ZADD")
		c.Error("wrong number", "ZADD", "s")
		c.Error("wrong number", "ZADD", "s", "1")
		c.Error("syntax error", "ZADD", "s", "1", "aap", "1")
		c.Error("not a valid float", "ZADD", "s", "nofloat", "aap")
		c.Error("wrong kind", "ZADD", "str", "1", "aap")
		c.Error("wrong number", "ZCARD")
		c.Error("wrong number", "ZCARD", "too", "many")
		c.Error("wrong kind", "ZCARD", "str")
		c.Error("wrong number", "ZRANK")
		c.Error("wrong number", "ZRANK", "key")
		c.Error("syntax error", "ZRANK", "key", "too", "many")
		c.Error("wrong kind", "ZRANK", "str", "member")
		c.Error("wrong number", "ZREVRANK")
		c.Error("wrong number", "ZREVRANK", "key")
		c.Error("wrong number", "ZPOPMAX")
		c.Error("out of range", "ZPOPMAX", "set", "noint")
		c.Error("syntax error", "ZPOPMAX", "set", "1", "toomany")
		c.Error("wrong number", "ZPOPMIN")
		c.Error("out of range", "ZPOPMIN", "set", "noint")
		c.Error("syntax error", "ZPOPMIN", "set", "1", "toomany")
		c.Error("syntax error", "ZRANK", "z", "nosuch", "WITHSCORES")
		c.Error("syntax error", "ZREVRANK", "z", "nosuch", "WITHSCORES")

		c.Do("RENAME", "z", "z2")
		c.Do("EXISTS", "z")
		c.Do("EXISTS", "z2")
		c.Do("MOVE", "z2", "3")
		c.Do("EXISTS", "z2")
		c.Do("SELECT", "3")
		c.Do("EXISTS", "z2")
		c.Do("DEL", "z2")
		c.Do("EXISTS", "z2")
	})

	testRaw(t, func(c *client) {
		c.Do("ZADD", "z", "0", "new\nline\n")
		c.Do("ZADD", "z", "0", "line")
		c.Do("ZADD", "z", "0", "another\nnew\nline\n")
		c.Do("ZSCAN", "z", "0", "MATCH", "*")
		c.Do("ZRANGEBYLEX", "z", "[a", "[z")
		c.Do("ZRANGE", "z", "0", "-1", "WITHSCORES")
	})

	testRaw(t, func(c *client) {
		// very small values
		c.Do("ZADD", "a_zset", "1.2", "one")
		c.Do("ZADD", "a_zset", "incr", "1.2", "one")
		c.DoRounded(1, "ZADD", "a_zset", "incr", "1.2", "one") // real: 3.5999999999999996, mini: 3.6
		c.Do("ZADD", "a_zset", "incr", "1.2", "one")
	})
}

func TestSortedSetAdd(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ZADD", "z",
			"1", "aap",
			"2", "noot",
		)
		c.Do("ZADD", "z", "NX",
			"1.1", "aap",
			"3", "mies",
		)
		c.Do("ZADD", "z", "XX",
			"1.2", "aap",
			"4", "vuur",
		)
		c.Do("ZADD", "z", "CH",
			"1.2", "aap",
			"4.1", "vuur",
			"5", "roos",
		)
		c.Do("ZADD", "z", "CH", "XX",
			"1.2", "aap",
			"4.2", "vuur",
			"5", "roos",
			"5", "zand",
		)
		c.Do("ZADD", "z", "XX", "XX", "XX", "XX",
			"1.2", "aap",
		)
		c.Do("ZADD", "z", "NX", "NX", "NX", "NX",
			"1.2", "aap",
		)
		c.Error("not compatible", "ZADD", "z", "XX", "NX", "1.1", "foo")
		c.Error("wrong number", "ZADD", "z", "XX")
		c.Error("wrong number", "ZADD", "z", "NX")
		c.Error("wrong number", "ZADD", "z", "CH")
		c.Error("wrong number", "ZADD", "z", "??")
		c.Error("syntax error", "ZADD", "z", "1.2", "aap", "XX")
		c.Error("syntax error", "ZADD", "z", "1.2", "aap", "CH")
		c.Error("wrong number", "ZADD", "z")
	})

	testRaw(t, func(c *client) {
		c.Do("ZADD", "z", "INCR", "1", "aap")
		c.Do("ZADD", "z", "INCR", "1", "aap")
		c.Do("ZADD", "z", "INCR", "1", "aap")
		c.Do("ZADD", "z", "INCR", "-12", "aap")
		c.Do("ZADD", "z", "INCR", "INCR", "-12", "aap")
		c.Do("ZADD", "z", "CH", "INCR", "-12", "aap") // 'CH' is ignored
		c.Do("ZADD", "z", "INCR", "CH", "-12", "aap") // 'CH' is ignored
		c.Do("ZADD", "z", "INCR", "NX", "12", "aap")
		c.Do("ZADD", "z", "INCR", "XX", "12", "aap")
		c.Do("ZADD", "q", "INCR", "NX", "12", "aap")
		c.Do("ZADD", "q", "INCR", "XX", "12", "aap")

		c.Error("INCR option", "ZADD", "z", "INCR", "1", "aap", "2", "tiger")
		c.Error("syntax error", "ZADD", "z", "INCR", "-12")
		c.Error("syntax error", "ZADD", "z", "INCR", "-12", "aap", "NX")
	})

	testRaw(t, func(c *client) {
		c.Do("ZADD", "z", "1", "score")
		c.Do("ZADD", "z", "GT", "2", "score")
		c.Do("ZADD", "z", "LT", "1", "score")

		c.Error("ERR GT, LT, and/or NX options at the same time are not compatible", "ZADD", "z", "GT", "LT", "1", "score")
	})

	testRESP3(t, func(c *client) {
		c.Do("ZADD", "z", "INCR", "1", "aap")
	})
}

func TestSortedSetRange(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ZADD", "z",
			"1", "aap",
			"2", "noot",
			"3", "mies",
			"2", "nootagain",
			"3", "miesagain",
			"+Inf", "the stars",
			"+Inf", "more stars",
			"-Inf", "big bang",
		)
		c.Do("ZADD", "zs",
			"5", "berlin",
			"5", "lisbon",
			"5", "manila",
			"5", "budapest",
			"5", "london",
			"5", "singapore",
			"5", "amsterdam",
		)

		t.Run("plain", func(t *testing.T) {
			c.Do("ZRANGE", "z", "0", "-1")
			c.Do("ZRANGE", "z", "0", "10", "WITHSCORES", "WITHSCORES")
			c.Do("ZRANGE", "z", "0", "-1", "WiThScOrEs")
			c.Do("ZRANGE", "z", "0", "10")
			c.Do("ZRANGE", "z", "0", "2")
			c.Do("ZRANGE", "z", "2", "20")
			c.Do("ZRANGE", "z", "0", "-4")
			c.Do("ZRANGE", "z", "2", "-4")
			c.Do("ZRANGE", "z", "400", "-1")
			c.Do("ZRANGE", "z", "300", "-110")
			c.Do("ZRANGE", "z", "0", "-1", "REV")
			c.Error("not an integer", "ZRANGE", "z", "(0", "-1")
			c.Error("not an integer", "ZRANGE", "z", "0", "(-1")
			c.Error("combination", "ZRANGE", "z", "0", "-1", "LIMIT", "1", "2")
		})

		t.Run("byscore", func(t *testing.T) {
			c.Do("ZRANGE", "z", "0", "-1", "BYSCORE")
			c.Do("ZRANGE", "z", "0", "1000", "BYSCORE")
			c.Do("ZRANGE", "z", "1", "2", "BYSCORE")
			c.Do("ZRANGE", "z", "1", "(2", "BYSCORE")
			c.Do("ZRANGE", "z", "-inf", "+inf", "BYSCORE")
			c.Do("ZRANGE", "z", "-inf", "+inf", "BYSCORE", "REV")
			c.Do("ZRANGE", "z", "-inf", "+inf", "BYSCORE", "LIMIT", "0", "1")
			c.Do("ZRANGE", "z", "-inf", "+inf", "BYSCORE", "LIMIT", "1", "2")
			c.Do("ZRANGE", "z", "-inf", "+inf", "BYSCORE", "LIMIT", "0", "-1")
			c.Do("ZRANGE", "z", "-inf", "+inf", "BYSCORE", "REV", "LIMIT", "0", "1")
			c.Error("not a float", "ZRANGE", "z", "[1", "2", "BYSCORE")
		})

		t.Run("bylex", func(t *testing.T) {
			c.Do("ZRANGE", "zs", "[be", "(ma", "BYLEX")
			c.Do("ZRANGE", "zs", "[be", "+", "BYLEX")
			c.Do("ZRANGE", "zs", "-", "(ma", "BYLEX")
			c.Do("ZRANGE", "zs", "-", "+", "BYLEX")
			c.Do("ZRANGE", "zs", "[be", "(ma", "BYLEX", "REV")
			c.Do("ZRANGE", "zs", "-", "+", "BYLEX", "LIMIT", "0", "1")
			c.Do("ZRANGE", "zs", "-", "+", "BYLEX", "LIMIT", "1", "3")
			c.Do("ZRANGE", "zs", "-", "+", "BYLEX", "LIMIT", "1", "-1")
			c.Do("ZRANGE", "zs", "-", "+", "BYLEX", "LIMIT", "1", "-1", "REV")
			c.Error("syntax error", "ZRANGE", "z", "[be", "[ma", "BYSCORE", "BYLEX")
			c.Error("range item", "ZRANGE", "z", "be", "(ma", "BYLEX")
			c.Error("range item", "ZRANGE", "z", "(be", "ma", "BYLEX")
		})

		c.Do("ZADD", "zz",
			"0", "aap",
			"0", "Aap",
			"0", "AAP",
			"0", "aAP",
			"0", "aAp",
		)
		c.Do("ZRANGE", "zz", "0", "-1")

		// failure cases
		c.Error("wrong number", "ZRANGE")
		c.Error("wrong number", "ZRANGE", "foo")
		c.Error("wrong number", "ZRANGE", "foo", "1")
		c.Error("syntax error", "ZRANGE", "foo", "2", "3", "toomany")
		c.Error("syntax error", "ZRANGE", "foo", "2", "3", "WITHSCORES", "toomany")
		c.Error("not an integer", "ZRANGE", "foo", "noint", "3")
		c.Error("not an integer", "ZRANGE", "foo", "2", "noint")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "ZRANGE", "str", "300", "-110")
	})
}

func TestSortedSetRevRange(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ZADD", "z",
			"1", "aap",
			"2", "noot",
			"3", "mies",
			"2", "nootagain",
			"3", "miesagain",
			"+Inf", "the stars",
			"+Inf", "more stars",
			"-Inf", "big bang",
		)
		c.Do("ZREVRANGE", "z", "0", "-1")
		c.Do("ZREVRANGE", "z", "0", "-1", "WITHSCORES")
		c.Do("ZREVRANGE", "z", "0", "-1", "WITHSCORES", "WITHSCORES", "WITHSCORES")
		c.Do("ZREVRANGE", "z", "0", "-1", "WiThScOrEs")
		c.Do("ZREVRANGE", "z", "0", "-2")
		c.Do("ZREVRANGE", "z", "0", "-1000")
		c.Do("ZREVRANGE", "z", "2", "-2")
		c.Do("ZREVRANGE", "z", "400", "-1")
		c.Do("ZREVRANGE", "z", "300", "-110")
		c.Error("syntax", "ZREVRANGE", "z", "300", "-110", "REV")
		// failure cases
		c.Error("wrong number", "ZREVRANGE")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "ZREVRANGE", "str", "300", "-110")
	})
}

func TestSortedSetRem(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ZADD", "z",
			"1", "aap",
			"2", "noot",
			"3", "mies",
			"2", "nootagain",
			"3", "miesagain",
			"+Inf", "the stars",
			"+Inf", "more stars",
			"-Inf", "big bang",
		)
		c.Do("ZREM", "z", "nosuch")
		c.Do("ZREM", "z", "mies", "nootagain")
		c.Do("ZRANGE", "z", "0", "-1")

		// failure cases
		c.Error("wrong number", "ZREM")
		c.Error("wrong number", "ZREM", "foo")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "ZREM", "str", "member")
	})
}

func TestSortedSetRemRangeByLex(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ZADD", "z",
			"12", "zero kelvin",
			"12", "minusfour",
			"12", "one",
			"12", "oneone",
			"12", "two",
			"12", "zwei",
			"12", "three",
			"12", "drei",
			"12", "inf",
		)
		c.Do("ZRANGEBYLEX", "z", "-", "+")
		c.Do("ZREMRANGEBYLEX", "z", "[o", "(t")
		c.Do("ZRANGEBYLEX", "z", "-", "+")
		c.Do("ZREMRANGEBYLEX", "z", "-", "+")
		c.Do("ZRANGEBYLEX", "z", "-", "+")

		// failure cases
		c.Error("wrong number", "ZREMRANGEBYLEX")
		c.Error("wrong number", "ZREMRANGEBYLEX", "key")
		c.Error("wrong number", "ZREMRANGEBYLEX", "key", "[a")
		c.Error("wrong number", "ZREMRANGEBYLEX", "key", "[a", "[b", "c")
		c.Error("not valid string range", "ZREMRANGEBYLEX", "key", "!a", "[b")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "ZREMRANGEBYLEX", "str", "[a", "[b")
	})
}

func TestSortedSetRemRangeByRank(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ZADD", "z",
			"12", "zero kelvin",
			"12", "minusfour",
			"12", "one",
			"12", "oneone",
			"12", "two",
			"12", "zwei",
			"12", "three",
			"12", "drei",
			"12", "inf",
		)
		c.Do("ZREMRANGEBYRANK", "z", "-2", "-1")
		c.Do("ZRANGE", "z", "0", "-1")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf")
		c.Do("ZREMRANGEBYRANK", "z", "-2", "-1")
		c.Do("ZRANGE", "z", "0", "-1")
		c.Do("ZREMRANGEBYRANK", "z", "0", "-1")
		c.Do("EXISTS", "z")

		c.Do("ZREMRANGEBYRANK", "nosuch", "-2", "-1")

		// failure cases
		c.Error("wrong number", "ZREMRANGEBYRANK")
		c.Error("wrong number", "ZREMRANGEBYRANK", "key")
		c.Error("wrong number", "ZREMRANGEBYRANK", "key", "0")
		c.Error("not an integer", "ZREMRANGEBYRANK", "key", "noint", "-1")
		c.Error("not an integer", "ZREMRANGEBYRANK", "key", "0", "noint")
		c.Error("wrong number", "ZREMRANGEBYRANK", "key", "0", "1", "too many")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "ZREMRANGEBYRANK", "str", "0", "-1")
	})
}

func TestSortedSetRemRangeByScore(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ZADD", "z",
			"1", "aap",
			"2", "noot",
			"3", "mies",
			"2", "nootagain",
			"3", "miesagain",
			"+Inf", "the stars",
			"+Inf", "more stars",
			"-Inf", "big bang",
		)
		c.Do("ZREMRANGEBYSCORE", "z", "-inf", "(2")
		c.Do("ZRANGE", "z", "0", "-1")
		c.Do("ZREMRANGEBYSCORE", "z", "(1000", "(2000")
		c.Do("ZRANGE", "z", "0", "-1")
		c.Do("ZREMRANGEBYSCORE", "z", "-inf", "+inf")
		c.Do("EXISTS", "z")

		c.Do("ZREMRANGEBYSCORE", "nosuch", "-inf", "inf")

		// failure cases
		c.Error("wrong number", "ZREMRANGEBYSCORE")
		c.Error("wrong number", "ZREMRANGEBYSCORE", "key")
		c.Error("wrong number", "ZREMRANGEBYSCORE", "key", "0")
		c.Error("not a float", "ZREMRANGEBYSCORE", "key", "noint", "-1")
		c.Error("not a float", "ZREMRANGEBYSCORE", "key", "0", "noint")
		c.Error("wrong number", "ZREMRANGEBYSCORE", "key", "0", "1", "too many")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "ZREMRANGEBYSCORE", "str", "0", "-1")
	})
}

func TestSortedSetScore(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ZADD", "z",
			"1", "aap",
			"2", "noot",
			"3", "mies",
			"2", "nootagain",
			"3", "miesagain",
			"+Inf", "the stars",
		)
		c.Do("ZSCORE", "z", "mies")
		c.Do("ZSCORE", "z", "the stars")
		c.Do("ZSCORE", "z", "nosuch")
		c.Do("ZSCORE", "nosuch", "nosuch")

		// failure cases
		c.Error("wrong number", "ZSCORE")
		c.Error("wrong number", "ZSCORE", "foo")
		c.Error("wrong number", "ZSCORE", "foo", "too", "many")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "ZSCORE", "str", "member")
	})
}

func TestSortedSetRangeByScore(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ZADD", "z",
			"1", "aap",
			"2", "noot",
			"3", "mies",
			"2", "nootagain",
			"3", "miesagain",
			"+Inf", "the stars",
			"+Inf", "more stars",
			"-Inf", "big bang",
		)
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "LIMIT", "1", "2")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "LIMIT", "-1", "2")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "LIMIT", "1", "-2")
		c.Do("ZREVRANGEBYSCORE", "z", "inf", "-inf")
		c.Do("ZREVRANGEBYSCORE", "z", "inf", "-inf", "LIMIT", "1", "2")
		c.Do("ZREVRANGEBYSCORE", "z", "inf", "-inf", "LIMIT", "-1", "2")
		c.Do("ZREVRANGEBYSCORE", "z", "inf", "-inf", "LIMIT", "1", "-2")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "WITHSCORES")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "WiThScOrEs")
		c.Do("ZREVRANGEBYSCORE", "z", "-inf", "inf", "WITHSCORES", "LIMIT", "1", "2")
		c.Do("ZRANGEBYSCORE", "z", "0", "3")
		c.Do("ZRANGEBYSCORE", "z", "0", "inf")
		c.Do("ZRANGEBYSCORE", "z", "(1", "3")
		c.Do("ZRANGEBYSCORE", "z", "(1", "(3")
		c.Do("ZRANGEBYSCORE", "z", "1", "(3")
		c.Do("ZRANGEBYSCORE", "z", "1", "(3", "LIMIT", "0", "2")
		c.Do("ZRANGEBYSCORE", "foo", "2", "3", "LIMIT", "1", "2", "WITHSCORES")
		c.Do("ZCOUNT", "z", "-inf", "inf")
		c.Do("ZCOUNT", "z", "0", "3")
		c.Do("ZCOUNT", "z", "0", "inf")
		c.Do("ZCOUNT", "z", "(2", "inf")

		// Bunch of limit edge cases
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "LIMIT", "0", "7")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "LIMIT", "0", "8")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "LIMIT", "0", "9")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "LIMIT", "7", "0")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "LIMIT", "7", "1")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "LIMIT", "7", "2")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "LIMIT", "8", "0")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "LIMIT", "8", "1")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "LIMIT", "8", "2")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "LIMIT", "9", "2")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "LIMIT", "-1", "2")
		c.Do("ZRANGEBYSCORE", "z", "-inf", "inf", "LIMIT", "-1", "-1")

		// failure cases
		c.Error("wrong number", "ZRANGEBYSCORE")
		c.Error("wrong number", "ZRANGEBYSCORE", "foo")
		c.Error("wrong number", "ZRANGEBYSCORE", "foo", "1")
		c.Error("syntax error", "ZRANGEBYSCORE", "foo", "2", "3", "toomany")
		c.Error("syntax error", "ZRANGEBYSCORE", "foo", "2", "3", "WITHSCORES", "toomany")
		c.Error("not an integer", "ZRANGEBYSCORE", "foo", "2", "3", "LIMIT", "noint", "1")
		c.Error("not an integer", "ZRANGEBYSCORE", "foo", "2", "3", "LIMIT", "1", "noint")
		c.Error("syntax error", "ZREVRANGEBYSCORE", "z", "-inf", "inf", "WITHSCORES", "LIMIT", "1", "-2", "toomany")
		c.Error("not a float", "ZRANGEBYSCORE", "foo", "noint", "3")
		c.Error("not a float", "ZRANGEBYSCORE", "foo", "[4", "3")
		c.Error("not a float", "ZRANGEBYSCORE", "foo", "2", "noint")
		c.Error("not a float", "ZRANGEBYSCORE", "foo", "4", "[3")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "ZRANGEBYSCORE", "str", "300", "-110")

		c.Error("wrong number", "ZREVRANGEBYSCORE")
		c.Error("not a float", "ZREVRANGEBYSCORE", "foo", "[4", "3")
		c.Error("wrong kind", "ZREVRANGEBYSCORE", "str", "300", "-110")

		c.Error("wrong number", "ZCOUNT")
		c.Error("not a float", "ZCOUNT", "foo", "[4", "3")
		c.Error("wrong kind", "ZCOUNT", "str", "300", "-110")
	})

	// Issue #10
	testRaw(t, func(c *client) {
		c.Do("ZADD", "key", "3.3", "element")
		c.Do("ZRANGEBYSCORE", "key", "3.3", "3.3")
		c.Do("ZRANGEBYSCORE", "key", "4.3", "4.3")
		c.Do("ZREVRANGEBYSCORE", "key", "3.3", "3.3")
		c.Do("ZREVRANGEBYSCORE", "key", "4.3", "4.3")
	})
}

func TestSortedSetRangeByLex(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ZADD", "z",
			"12", "zero kelvin",
			"12", "minusfour",
			"12", "one",
			"12", "oneone",
			"12", "two",
			"12", "zwei",
			"12", "three",
			"12", "drei",
			"12", "inf",
		)
		c.Do("ZRANGEBYLEX", "z", "-", "+")
		c.Do("ZREVRANGEBYLEX", "z", "+", "-")
		c.Do("ZLEXCOUNT", "z", "-", "+")
		c.Do("ZRANGEBYLEX", "z", "[o", "[three")
		c.Do("ZREVRANGEBYLEX", "z", "[three", "[o")
		c.Do("ZLEXCOUNT", "z", "[o", "[three")
		c.Do("ZRANGEBYLEX", "z", "(o", "(z")
		c.Do("ZREVRANGEBYLEX", "z", "(z", "(o")
		c.Do("ZLEXCOUNT", "z", "(o", "(z")
		c.Do("ZRANGEBYLEX", "z", "+", "(z")
		c.Do("ZREVRANGEBYLEX", "z", "(z", "+")
		c.Do("ZRANGEBYLEX", "z", "(a", "-")
		c.Do("ZREVRANGEBYLEX", "z", "-", "(a")
		c.Do("ZRANGEBYLEX", "z", "(z", "(a")
		c.Do("ZREVRANGEBYLEX", "z", "(a", "(z")
		c.Do("ZRANGEBYLEX", "nosuch", "-", "+")
		c.Do("ZREVRANGEBYLEX", "nosuch", "+", "-")
		c.Do("ZLEXCOUNT", "nosuch", "-", "+")
		c.Do("ZRANGEBYLEX", "z", "-", "+", "LIMIT", "1", "2")
		c.Do("ZREVRANGEBYLEX", "z", "+", "-", "LIMIT", "1", "2")
		c.Do("ZRANGEBYLEX", "z", "-", "+", "LIMIT", "-1", "2")
		c.Do("ZREVRANGEBYLEX", "z", "+", "-", "LIMIT", "-1", "2")
		c.Do("ZRANGEBYLEX", "z", "-", "+", "LIMIT", "1", "-2")
		c.Do("ZREVRANGEBYLEX", "z", "+", "-", "LIMIT", "1", "-2")

		c.Do("ZADD", "z", "12", "z")
		c.Do("ZADD", "z", "12", "zz")
		c.Do("ZADD", "z", "12", "zzz")
		c.Do("ZADD", "z", "12", "zzzz")
		c.Do("ZRANGEBYLEX", "z", "[z", "+")
		c.Do("ZREVRANGEBYLEX", "z", "+", "[z")
		c.Do("ZRANGEBYLEX", "z", "(z", "+")
		c.Do("ZREVRANGEBYLEX", "z", "+", "(z")
		c.Do("ZLEXCOUNT", "z", "(z", "+")

		// failure cases
		c.Error("wrong number", "ZRANGEBYLEX")
		c.Error("wrong number", "ZREVRANGEBYLEX")
		c.Error("wrong number", "ZRANGEBYLEX", "key")
		c.Error("wrong number", "ZRANGEBYLEX", "key", "[a")
		c.Error("syntax error", "ZRANGEBYLEX", "key", "[a", "[b", "c")
		c.Error("not valid string range", "ZRANGEBYLEX", "key", "!a", "[b")
		c.Error("not valid string range", "ZRANGEBYLEX", "key", "[a", "!b")
		c.Error("not valid string range", "ZRANGEBYLEX", "key", "[a", "b]")
		c.Error("not valid string range item", "ZRANGEBYLEX", "key", "[a", "")
		c.Error("not valid string range item", "ZRANGEBYLEX", "key", "", "[b")
		c.Error("syntax error", "ZRANGEBYLEX", "key", "[a", "[b", "LIMIT")
		c.Error("syntax error", "ZRANGEBYLEX", "key", "[a", "[b", "LIMIT", "1")
		c.Error("not an integer", "ZRANGEBYLEX", "key", "[a", "[b", "LIMIT", "a", "1")
		c.Error("not an integer", "ZRANGEBYLEX", "key", "[a", "[b", "LIMIT", "1", "a")
		c.Error("syntax error", "ZRANGEBYLEX", "key", "[a", "[b", "LIMIT", "1", "1", "toomany")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "ZRANGEBYLEX", "str", "[a", "[b")

		c.Error("wrong number", "ZLEXCOUNT")
		c.Error("wrong number", "ZLEXCOUNT", "key")
		c.Error("wrong number", "ZLEXCOUNT", "key", "[a")
		c.Error("wrong number", "ZLEXCOUNT", "key", "[a", "[b", "c")
		c.Error("not valid string range", "ZLEXCOUNT", "key", "!a", "[b")
		c.Error("wrong kind", "ZLEXCOUNT", "str", "[a", "[b")
	})

	testRaw(t, func(c *client) {
		c.Do("ZADD", "idx", "0", "ccc")
		c.Do("ZRANGEBYLEX", "idx", "[d", "[e")
		c.Do("ZRANGEBYLEX", "idx", "[c", "[d")
	})
}

func TestSortedSetIncyby(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ZINCRBY", "z", "1.0", "m")
		c.Do("ZINCRBY", "z", "1.0", "m")
		c.Do("ZINCRBY", "z", "1.0", "m")
		c.Do("ZINCRBY", "z", "2.0", "m")
		c.Do("ZINCRBY", "z", "3", "m2")
		c.Do("ZINCRBY", "z", "3", "m2")
		c.Do("ZINCRBY", "z", "3", "m2")

		// failure cases
		c.Error("wrong number", "ZINCRBY")
		c.Error("wrong number", "ZINCRBY", "key")
		c.Error("wrong number", "ZINCRBY", "key", "1.0")
		c.Error("not a valid float", "ZINCRBY", "key", "nofloat", "m")
		c.Error("wrong number", "ZINCRBY", "key", "1.0", "too", "many")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "ZINCRBY", "str", "1.0", "member")
	})
}

func TestZscan(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		// No set yet
		c.Do("ZSCAN", "h", "0")

		c.Do("ZADD", "h", "1.0", "key1")
		c.Do("ZSCAN", "h", "0")
		c.Do("ZSCAN", "h", "0", "COUNT", "12")
		c.Do("ZSCAN", "h", "0", "cOuNt", "12")

		// ZSCAN may return a higher count of items than requested (See https://redis.io/docs/manual/keyspace/), so we must query all items.
		c.Do("ZSCAN", "h", "0", "COUNT", "10") // cursor differs

		c.Do("ZADD", "h", "2.0", "anotherkey")
		c.Do("ZSCAN", "h", "0", "MATCH", "anoth*")
		c.Do("ZSCAN", "h", "0", "MATCH", "anoth*", "COUNT", "100")
		c.Do("ZSCAN", "h", "0", "COUNT", "100", "MATCH", "anoth*")

		// Can't really test multiple keys.
		// c.Do("SET", "key2", "value2")
		// c.Do("SCAN", "0")

		// Error cases
		c.Error("wrong number", "ZSCAN")
		c.Error("wrong number", "ZSCAN", "noint")
		c.Error("not an integer", "ZSCAN", "h", "0", "COUNT", "noint")
		c.Error("syntax error", "ZSCAN", "h", "0", "COUNT")
		c.Error("syntax error", "ZSCAN", "h", "0", "COUNT", "0")
		c.Error("syntax error", "ZSCAN", "h", "0", "COUNT", "-1")
		c.Error("syntax error", "ZSCAN", "h", "0", "MATCH")
		c.Error("syntax error", "ZSCAN", "h", "0", "garbage")
		c.Error("syntax error", "ZSCAN", "h", "0", "COUNT", "12", "MATCH", "foo", "garbage")
		// c.Do("ZSCAN", "nosuch", "0", "COUNT", "garbage")
		c.Do("SET", "str", "1")
		c.Error("wrong kind", "ZSCAN", "str", "0")
	})
}

func TestZunion(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		// example from the docs https://redis.io/commands/ZUNION
		c.Do("ZADD", "zset1", "1", "one")
		c.Do("ZADD", "zset1", "2", "two")
		c.Do("ZADD", "zset2", "1", "one")
		c.Do("ZADD", "zset2", "2", "two")
		c.Do("ZADD", "zset2", "3", "three")
		c.Do("ZUNION", "2", "zset1", "zset2")
		c.Do("ZUNION", "2", "zset1", "zset2", "WITHSCORES")
	})
	testRaw(t, func(c *client) {
		c.Do("ZADD", "h1", "1.0", "key1")
		c.Do("ZADD", "h1", "2.0", "key2")
		c.Do("ZADD", "h2", "1.0", "key1")
		c.Do("ZADD", "h2", "4.0", "key2")
		c.Do("ZUNION", "2", "h1", "h2", "WITHSCORES")

		c.Do("ZUNION", "2", "h1", "h2", "WEIGHTS", "2.0", "12", "WITHSCORES")
		c.Do("ZUNION", "2", "h1", "h2", "WEIGHTS", "2", "-12", "WITHSCORES")

		c.Do("ZUNION", "2", "h1", "h2", "AGGREGATE", "min", "WITHSCORES")
		c.Do("ZUNION", "2", "h1", "h2", "AGGREGATE", "max", "WITHSCORES")
		c.Do("ZUNION", "2", "h1", "h2", "AGGREGATE", "sum", "WITHSCORES")

		// Error cases
		c.Error("wrong number", "ZUNION")
		c.Error("wrong number", "ZUNION", "noint")
		c.Error("at least 1", "ZUNION", "0", "f")
		c.Error("syntax error", "ZUNION", "2", "f")
		c.Error("at least 1", "ZUNION", "-1", "f")
		c.Error("syntax error", "ZUNION", "2", "f1", "f2", "f3")
		c.Error("syntax error", "ZUNION", "2", "f1", "f2", "WEIGHTS")
		c.Error("syntax error", "ZUNION", "2", "f1", "f2", "WEIGHTS", "1")
		c.Error("syntax error", "ZUNION", "2", "f1", "f2", "WEIGHTS", "1", "2", "3")
		c.Error("not a float", "ZUNION", "2", "f1", "f2", "WEIGHTS", "f", "2")
		c.Error("syntax error", "ZUNION", "2", "f1", "f2", "AGGREGATE", "foo")
		c.Do("SET", "str", "1")
		c.Error("wrong kind", "ZUNION", "1", "str")
	})
	// not a sorted set, still fine
	testRaw(t, func(c *client) {
		c.Do("SADD", "super", "1", "2", "3")
		c.Do("SADD", "exclude", "3")
		c.Do("ZUNION", "2", "super", "exclude", "weights", "1", "0", "aggregate", "min", "withscores")
	})
}

func TestZunionstore(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ZADD", "h1", "1.0", "key1")
		c.Do("ZADD", "h1", "2.0", "key2")
		c.Do("ZADD", "h2", "1.0", "key1")
		c.Do("ZADD", "h2", "4.0", "key2")
		c.Do("ZUNIONSTORE", "res", "2", "h1", "h2")
		c.Do("ZRANGE", "res", "0", "-1", "WITHSCORES")

		c.Do("ZUNIONSTORE", "weighted", "2", "h1", "h2", "WEIGHTS", "2.0", "12")
		c.Do("ZRANGE", "weighted", "0", "-1", "WITHSCORES")
		c.Do("ZUNIONSTORE", "weighted2", "2", "h1", "h2", "WEIGHTS", "2", "-12")
		c.Do("ZRANGE", "weighted2", "0", "-1", "WITHSCORES")

		c.Do("ZUNIONSTORE", "amin", "2", "h1", "h2", "AGGREGATE", "min")
		c.Do("ZRANGE", "amin", "0", "-1", "WITHSCORES")
		c.Do("ZUNIONSTORE", "amax", "2", "h1", "h2", "AGGREGATE", "max")
		c.Do("ZRANGE", "amax", "0", "-1", "WITHSCORES")
		c.Do("ZUNIONSTORE", "asum", "2", "h1", "h2", "AGGREGATE", "sum")
		c.Do("ZRANGE", "asum", "0", "-1", "WITHSCORES")

		// Error cases
		c.Error("wrong number", "ZUNIONSTORE")
		c.Error("wrong number", "ZUNIONSTORE", "h")
		c.Error("wrong number", "ZUNIONSTORE", "h", "noint")
		c.Error("at least 1", "ZUNIONSTORE", "h", "0", "f")
		c.Error("syntax error", "ZUNIONSTORE", "h", "2", "f")
		c.Error("at least 1", "ZUNIONSTORE", "h", "-1", "f")
		c.Error("syntax error", "ZUNIONSTORE", "h", "2", "f1", "f2", "f3")
		c.Error("syntax error", "ZUNIONSTORE", "h", "2", "f1", "f2", "WEIGHTS")
		c.Error("syntax error", "ZUNIONSTORE", "h", "2", "f1", "f2", "WEIGHTS", "1")
		c.Error("syntax error", "ZUNIONSTORE", "h", "2", "f1", "f2", "WEIGHTS", "1", "2", "3")
		c.Error("not a float", "ZUNIONSTORE", "h", "2", "f1", "f2", "WEIGHTS", "f", "2")
		c.Error("syntax error", "ZUNIONSTORE", "h", "2", "f1", "f2", "AGGREGATE", "foo")
		c.Do("SET", "str", "1")
		c.Error("wrong kind", "ZUNIONSTORE", "h", "1", "str")
	})
	// overwrite
	testRaw(t, func(c *client) {
		c.Do("ZADD", "h1", "1.0", "key1")
		c.Do("ZADD", "h1", "2.0", "key2")
		c.Do("ZADD", "h2", "1.0", "key1")
		c.Do("ZADD", "h2", "4.0", "key2")
		c.Do("SET", "str", "1")
		c.Do("ZUNIONSTORE", "str", "2", "h1", "h2")
		c.Do("TYPE", "str")
		c.Do("ZUNIONSTORE", "h2", "2", "h1", "h2")
		c.Do("ZRANGE", "h2", "0", "-1", "WITHSCORES")
		c.Do("TYPE", "h1")
		c.Do("TYPE", "h2")
	})
	// not a sorted set, still fine
	testRaw(t, func(c *client) {
		c.Do("SADD", "super", "1", "2", "3")
		c.Do("SADD", "exclude", "3")
		c.Do("ZUNIONSTORE", "tmp", "2", "super", "exclude", "weights", "1", "0", "aggregate", "min")
		c.Do("ZRANGE", "tmp", "0", "-1", "withscores")
	})
}

func TestZinter(t *testing.T) {
	skip(t)
	// ZINTER
	testRaw(t, func(c *client) {
		c.Do("ZADD", "h1", "1.0", "key1")
		c.Do("ZADD", "h1", "2.0", "key2")
		c.Do("ZADD", "h1", "3.0", "key3")
		c.Do("ZADD", "h2", "1.0", "key1")
		c.Do("ZADD", "h2", "4.0", "key2")
		c.Do("ZADD", "h3", "4.0", "key4")
		c.DoSorted("ZINTER", "2", "h1", "h2")

		c.DoSorted("ZINTER", "2", "h1", "h2", "WEIGHTS", "2.0", "12")
		c.DoSorted("ZINTER", "2", "h1", "h2", "WEIGHTS", "2", "-12")

		c.DoSorted("ZINTER", "2", "h1", "h2", "AGGREGATE", "min")
		c.DoSorted("ZINTER", "2", "h1", "h2", "AGGREGATE", "max")
		c.DoSorted("ZINTER", "2", "h1", "h2", "AGGREGATE", "sum")

		// normal set
		c.Do("ZADD", "q1", "2", "f1")
		c.Do("SADD", "q2", "f1")
		c.Do("ZINTER", "2", "q1", "q2")
		c.DoSorted("ZINTER", "2", "q1", "q2", "WITHSCORES")

		// Error cases
		c.Error("wrong number", "ZINTER")
		c.Error("wrong number", "ZINTER", "noint")
		c.Error("at least 1", "ZINTER", "0", "f")
		c.Error("syntax error", "ZINTER", "2", "f")
		c.Error("at least 1", "ZINTER", "-1", "f")
		c.Error("syntax error", "ZINTER", "2", "f1", "f2", "f3")
		c.Error("syntax error", "ZINTER", "2", "f1", "f2", "WEIGHTS")
		c.Error("syntax error", "ZINTER", "2", "f1", "f2", "WEIGHTS", "1")
		c.Error("syntax error", "ZINTER", "2", "f1", "f2", "WEIGHTS", "1", "2", "3")
		c.Error("not a float", "ZINTER", "2", "f1", "f2", "WEIGHTS", "f", "2")
		c.Error("syntax error", "ZINTER", "2", "f1", "f2", "AGGREGATE", "foo")
		c.Do("SET", "str", "1")
		c.Error("wrong kind", "ZINTER", "1", "str")
	})

	// ZINTERSTORE
	testRaw(t, func(c *client) {
		c.Do("ZADD", "h1", "1.0", "key1")
		c.Do("ZADD", "h1", "2.0", "key2")
		c.Do("ZADD", "h1", "3.0", "key3")
		c.Do("ZADD", "h2", "1.0", "key1")
		c.Do("ZADD", "h2", "4.0", "key2")
		c.Do("ZADD", "h3", "4.0", "key4")
		c.Do("ZINTERSTORE", "res", "2", "h1", "h2")
		c.Do("ZRANGE", "res", "0", "-1", "WITHSCORES")

		c.Do("ZINTERSTORE", "weighted", "2", "h1", "h2", "WEIGHTS", "2.0", "12")
		c.Do("ZRANGE", "weighted", "0", "-1", "WITHSCORES")
		c.Do("ZINTERSTORE", "weighted2", "2", "h1", "h2", "WEIGHTS", "2", "-12")
		c.Do("ZRANGE", "weighted2", "0", "-1", "WITHSCORES")

		c.Do("ZINTERSTORE", "amin", "2", "h1", "h2", "AGGREGATE", "min")
		c.Do("ZRANGE", "amin", "0", "-1", "WITHSCORES")
		c.Do("ZINTERSTORE", "amax", "2", "h1", "h2", "AGGREGATE", "max")
		c.Do("ZRANGE", "amax", "0", "-1", "WITHSCORES")
		c.Do("ZINTERSTORE", "asum", "2", "h1", "h2", "AGGREGATE", "sum")
		c.Do("ZRANGE", "asum", "0", "-1", "WITHSCORES")

		// normal set
		c.Do("ZADD", "q1", "2", "f1")
		c.Do("SADD", "q2", "f1")
		c.Do("ZINTERSTORE", "dest", "2", "q1", "q2")
		c.Do("ZRANGE", "dest", "0", "-1", "withscores")

		// store into self
		c.Do("ZINTERSTORE", "q1", "2", "q1", "q2")
		c.Do("ZRANGE", "q1", "0", "-1", "withscores")
		c.Do("SMEMBERS", "q2")

		// Error cases
		c.Error("wrong number", "ZINTERSTORE")
		c.Error("wrong number", "ZINTERSTORE", "h")
		c.Error("wrong number", "ZINTERSTORE", "h", "noint")
		c.Error("at least 1", "ZINTERSTORE", "h", "0", "f")
		c.Error("syntax error", "ZINTERSTORE", "h", "2", "f")
		c.Error("at least 1", "ZINTERSTORE", "h", "-1", "f")
		c.Error("syntax error", "ZINTERSTORE", "h", "2", "f1", "f2", "f3")
		c.Error("syntax error", "ZINTERSTORE", "h", "2", "f1", "f2", "WEIGHTS")
		c.Error("syntax error", "ZINTERSTORE", "h", "2", "f1", "f2", "WEIGHTS", "1")
		c.Error("syntax error", "ZINTERSTORE", "h", "2", "f1", "f2", "WEIGHTS", "1", "2", "3")
		c.Error("not a float", "ZINTERSTORE", "h", "2", "f1", "f2", "WEIGHTS", "f", "2")
		c.Error("syntax error", "ZINTERSTORE", "h", "2", "f1", "f2", "AGGREGATE", "foo")
		c.Do("SET", "str", "1")
		c.Error("wrong kind", "ZINTERSTORE", "h", "1", "str")
	})
}

func TestZpopminmax(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ZADD", "set:zpop", "1.0", "key1")
		c.Do("ZADD", "set:zpop", "2.0", "key2")
		c.Do("ZADD", "set:zpop", "3.0", "key3")
		c.Do("ZADD", "set:zpop", "4.0", "key4")
		c.Do("ZADD", "set:zpop", "5.0", "key5")
		c.Do("ZCARD", "set:zpop")

		c.Do("ZSCORE", "set:zpop", "key1")
		c.Do("ZSCORE", "set:zpop", "key5")

		c.Do("ZPOPMIN", "set:zpop")
		c.Do("ZPOPMIN", "set:zpop", "2")
		c.Do("ZPOPMIN", "set:zpop", "100")
		c.Error("out of range", "ZPOPMIN", "set:zpop", "-100")

		c.Do("ZPOPMAX", "set:zpop")
		c.Do("ZPOPMAX", "set:zpop", "2")
		c.Do("ZPOPMAX", "set:zpop", "100")
		c.Error("out of range", "ZPOPMAX", "set:zpop", "-100")
		c.Do("ZPOPMAX", "nosuch", "1")

		// Wrong args
		c.Error("wrong number", "ZPOPMIN")
		c.Error("out of range", "ZPOPMIN", "set:zpop", "h1")
		c.Error("syntax error", "ZPOPMIN", "set:zpop", "1", "h2")
	})
}

func TestZrandmember(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ZADD", "q", "1.0", "key1")
		c.Do("ZADD", "q", "2.0", "key2")
		c.Do("ZADD", "q", "3.0", "key3")
		c.Do("ZADD", "q", "4.0", "key4")
		c.Do("ZADD", "q", "5.0", "key5")
		c.Do("ZCARD", "q")

		c.DoLoosely("ZRANDMEMBER", "q")

		c.DoLoosely("ZRANDMEMBER", "q", "3")
		c.DoLoosely("ZRANDMEMBER", "q", "4")
		c.DoLoosely("ZRANDMEMBER", "q", "5")
		c.DoLoosely("ZRANDMEMBER", "q", "6")
		c.DoLoosely("ZRANDMEMBER", "q", "7")
		c.DoLoosely("ZRANDMEMBER", "q", "12")
		c.Do("ZRANDMEMBER", "q", "0")
		c.DoLoosely("ZRANDMEMBER", "q", "-3")
		c.DoLoosely("ZRANDMEMBER", "q", "-4")
		c.DoLoosely("ZRANDMEMBER", "q", "-5")
		c.DoLoosely("ZRANDMEMBER", "q", "-6")
		c.DoLoosely("ZRANDMEMBER", "q", "-7")
		c.DoLoosely("ZRANDMEMBER", "q", "-12")
		c.Do("ZRANDMEMBER", "nosuch")
		c.Do("ZRANDMEMBER", "nosuch", "4")
		c.Do("ZRANDMEMBER", "nosuch", "-4")
		c.DoLoosely("ZRANDMEMBER", "q", "2", "WITHSCORES")
		c.DoLoosely("ZRANDMEMBER", "q", "0", "WITHSCORES")
		c.DoLoosely("ZRANDMEMBER", "q", "-2", "WITHSCORES")
		c.DoLoosely("ZRANDMEMBER", "nosuch", "2", "WITHSCORES")
		c.DoLoosely("ZRANDMEMBER", "nosuch", "-2", "WITHSCORES")

		// Wrong args
		c.Error("wrong number", "ZRANDMEMBER")
		c.Do("SET", "str", "1")
		c.Error("wrong kind", "ZRANDMEMBER", "str")
		c.Error("not an integer", "ZRANDMEMBER", "q", "two")
	})
}

func TestZMScore(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ZADD", "q", "1.0", "key1")
		c.Do("ZADD", "q", "2.0", "key2")
		c.Do("ZADD", "q", "3.0", "key3")
		c.Do("ZADD", "q", "4.0", "key4")
		c.Do("ZADD", "q", "5.0", "key5")

		c.Do("ZMSCORE", "q", "key1")
		c.Do("ZMSCORE", "q", "key1 key2 key3")
		c.Do("ZMSCORE", "q", "nosuch")
		c.Do("ZMSCORE", "nosuch", "key1")
		c.Do("ZMSCORE", "nosuch", "key1", "key2")

		// failure cases
		c.Error("wrong number", "ZMSCORE", "q")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "ZMSCORE", "str", "key1")
	})
}
