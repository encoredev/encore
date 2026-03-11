package main

import (
	"strconv"
	"testing"
)

func TestString(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "foo", "bar")
		c.Do("GET", "foo")
		c.Do("SET", "foo", "bar\bbaz")
		c.Do("GET", "foo")
		c.Do("SET", "foo", "bar", "EX", "100")
		c.Error("not an integer", "SET", "foo", "bar", "EX", "noint")
		c.Do("SET", "utf8", "❆❅❄☃")
		c.Do("SET", "foo", "baz", "KEEPTTL")
		c.Do("SET", "foo", "bar", "GET")
		c.Do("SET", "new", "bar", "GET")
		c.Do("SET", "empty", "", "GET")
		c.Do("SET", "empty", "filled", "GET")
		c.Do("SET", "empty", "", "GET")

		c.Do("SET", "fooexat", "bar", "EXAT", "2345678901")
		c.DoApprox(10, "TTL", "fooexat")
		c.Error("not an integer", "SET", "foo", "bar", "EXAT", "noint")
		c.Do("SET", "foopxat", "bar", "PXAT", "2345678901000")
		c.DoApprox(10, "TTL", "foopxat")
		c.Error("not an integer", "SET", "foo", "bar", "PXAT", "noint")
		// expires right away
		c.Do("SET", "gone", "bar", "EXAT", "123")
		c.Do("EXISTS", "gone")

		// SET NX GET
		c.Do("SET", "unique", "value1", "NX", "GET")
		c.Do("SET", "unique", "value2", "NX", "GET")
		c.Do("SET", "unique", "value3", "XX", "GET")
		c.Do("SET", "unique", "value4", "XX", "GET")
		c.Do("SET", "uniquer", "value5", "XX", "GET")

		// Failure cases
		c.Error("wrong number", "SET")
		c.Error("wrong number", "SET", "foo")
		c.Error("syntax error", "SET", "foo", "bar", "baz")
		c.Error("wrong number", "GET")
		c.Error("wrong number", "GET", "too", "many")
		c.Error("invalid expire", "SET", "foo", "bar", "EX", "0")
		c.Error("invalid expire", "SET", "foo", "bar", "EX", "-100")
		c.Error("syntax error", "SET", "both", "bar", "PXAT", "3345678901000", "EXAT", "2345678901")
		c.Error("invalid expire", "SET", "foo", "bar", "EXAT", "-100")
		c.Error("invalid expire", "SET", "foo", "bar", "PXAT", "-100")
		c.Error("syntax error", "SET", "both", "bar", "PX", "6", "EX", "6")
		c.Error("syntax error", "SET", "both", "bar", "PX", "6", "EX", "0")
		c.Error("syntax error", "SET", "both", "bar", "PX", "6", "PXAT", "2345678901")
		// Wrong type
		c.Do("HSET", "hash", "key", "value")
		c.Error("wrong kind", "GET", "hash")
		c.Error("wrong kind", "SET", "hash", "foo", "GET")
	})
}

func TestStringGetSet(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "foo", "bar")
		c.Do("GETSET", "foo", "new")
		c.Do("GET", "foo")
		c.Do("GET", "new")
		c.Do("GETSET", "nosuch", "new")
		c.Do("GET", "nosuch")

		// Failure cases
		c.Error("wrong number", "GETSET")
		c.Error("wrong number", "GETSET", "foo")
		c.Error("wrong number", "GETSET", "foo", "bar", "baz")
		// Wrong type
		c.Do("HSET", "hash", "key", "value")
		c.Error("wrong kind", "GETSET", "hash", "new")
	})
}

func TestStringGetex(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("GETEX", "missing")

		c.Do("SET", "foo", "bar")
		c.Do("GETEX", "foo")
		c.Do("TTL", "foo")

		c.Do("GETEX", "foo", "EX", "10")
		c.Do("TTL", "foo")

		// Failure cases
		c.Error("wrong number", "GETEX")
		c.Error("syntax error", "GETEX", "foo", "bar")
		c.Error("syntax error", "GETEX", "foo", "EX", "10", "PERSIST")
		c.Error("syntax error", "GETEX", "foo", "EX", "10", "PX", "10")
		c.Error("not an integer", "GETEX", "foo", "EX", "ten")

		// Wrong type
		c.Do("HSET", "hash", "key", "value")
		c.Error("wrong kind", "GETEX", "hash")

		c.Do("SET", "hittl", "bar")
		c.Do("PEXPIRE", "hittl", "999999")
		c.Do("GETEX", "hittl", "PERSIST")
		c.Do("TTL", "hittl")
	})
}

func TestStringGetdel(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("GETDEL", "missing")

		c.Do("SET", "foo", "bar")
		c.Do("GETDEL", "foo")
		c.Do("EXISTS", "foo")

		// Failure cases
		c.Error("wrong number", "GETDEL")
		c.Error("wrong number", "GETDEL", "foo", "bar")
		// Wrong type
		c.Do("HSET", "hash", "key", "value")
		c.Error("wrong kind", "GETDEL", "hash")
	})
}

func TestStringMget(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "foo", "bar")
		c.Do("SET", "foo2", "bar")
		c.Do("MGET", "foo")
		c.Do("MGET", "foo", "foo2")
		c.Do("MGET", "nosuch", "neither")
		c.Do("MGET", "nosuch", "neither", "foo")

		// Failure cases
		c.Error("wrong number", "MGET")
		// Wrong type
		c.Do("HSET", "hash", "key", "value")
		c.Do("MGET", "hash") // not an error.
	})
}

func TestStringSetnx(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SETNX", "foo", "bar")
		c.Do("GET", "foo")
		c.Do("SETNX", "foo", "bar2")
		c.Do("GET", "foo")

		// Failure cases
		c.Error("wrong number", "SETNX")
		c.Error("wrong number", "SETNX", "foo")
		c.Error("wrong number", "SETNX", "foo", "bar", "baz")
		// Wrong type
		c.Do("HSET", "hash", "key", "value")
		c.Do("SETNX", "hash", "value")
	})
}

func TestExpire(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("EXPIRETIME", "missing")
		c.Do("PEXPIRETIME", "missing")

		c.Do("SET", "foo", "bar")
		c.Do("EXPIRETIME", "foo")
		c.Do("PEXPIRETIME", "foo")

		c.Do("EXPIRE", "foo", "12")
		c.Do("TTL", "foo")
		c.Do("TTL", "nosuch")
		c.Do("SET", "foo", "bar")
		c.Do("PEXPIRE", "foo", "999999")
		c.Do("EXPIREAT", "foo", "2234567890")
		c.Do("PEXPIREAT", "foo", "2234567890123")
		c.Do("EXPIRETIME", "foo")
		c.Do("PEXPIRETIME", "foo")
		// c.Do("PTTL", "foo")
		c.Do("PTTL", "nosuch")

		c.Do("SET", "foo", "bar")
		c.Do("EXPIRE", "foo", "0")
		c.Do("EXISTS", "foo")
		c.Do("SET", "foo", "bar")
		c.Do("EXPIRE", "foo", "-12")
		c.Do("EXISTS", "foo")

		c.Do("SET", "gt", "nice day today, right?")
		c.Do("EXPIRE", "gt", "10", "GT")
		c.Do("TTL", "gt")
		c.Do("EXPIRE", "gt", "10", "LT")
		c.Do("TTL", "gt")
		c.Do("EXPIRE", "gt", "3", "GT")
		c.Do("TTL", "gt")
		c.Do("EXPIRE", "gt", "999", "NX")
		c.Do("TTL", "gt")
		c.Do("EXPIRE", "gt", "999", "XX")
		c.Do("TTL", "gt")
		c.Do("PEXPIRE", "gt", "999000", "XX")
		c.Do("TTL", "gt")

		c.Do("SET", "pgt", "indeed it is")
		c.Do("PEXPIRE", "pgt", "10000", "LT", "XX")
		c.Do("TTL", "pgt")

		c.Error("wrong number", "EXPIRE")
		c.Error("wrong number", "EXPIRE", "foo")
		c.Error("not an integer", "EXPIRE", "foo", "noint")
		c.Error("Unsupported", "EXPIRE", "foo", "12", "invaLID")
		c.Error("Unsupported", "EXPIRE", "foo", "12", "GT", "toomany")
		c.Error("at the same time", "EXPIRE", "foo", "12", "GT", "LT")
		c.Error("at the same time", "EXPIRE", "foo", "12", "LT", "NX")
		c.Error("wrong number", "EXPIREAT")
		c.Error("wrong number", "TTL")
		c.Error("wrong number", "TTL", "too", "many")
		c.Error("wrong number", "PEXPIRE")
		c.Error("wrong number", "PEXPIRE", "foo")
		c.Error("not an integer", "PEXPIRE", "foo", "noint")
		c.Error("Unsupported", "PEXPIRE", "foo", "12", "toomany")
		c.Error("Unsupported", "PEXPIREAT", "foo", "12", "NX", "toomany")
		c.Error("wrong number", "PEXPIREAT")
		c.Error("wrong number", "PTTL")
		c.Error("wrong number", "PTTL", "too", "many")

		c.Error("wrong number", "EXPIRETIME")
		c.Error("wrong number", "EXPIRETIME", "too", "many")
		c.Error("wrong number", "PEXPIRETIME")
		c.Error("wrong number", "PEXPIRETIME", "too", "many")
	})
}

func TestMset(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("MSET", "foo", "bar")
		c.Do("MSET", "foo", "bar", "baz", "?")
		c.Do("MSET", "foo", "bar", "foo", "baz") // double key
		c.Do("GET", "foo")
		// Error cases
		c.Error("wrong number", "MSET")
		c.Error("wrong number", "MSET", "foo")
		c.Error("wrong number", "MSET", "foo", "bar", "baz")

		c.Do("MSETNX", "foo", "bar", "aap", "noot")
		c.Do("MSETNX", "one", "two", "three", "four")
		c.Do("MSETNX", "11", "12", "11", "14") // double key
		c.Do("GET", "11")

		// Wrong type of key doesn't matter
		c.Do("HSET", "aap", "noot", "mies")
		c.Do("MSET", "aap", "again", "eight", "nine")
		c.Do("MSETNX", "aap", "again", "eight", "nine")

		// Error cases
		c.Error("wrong number", "MSETNX")
		c.Error("wrong number", "MSETNX", "one")
		c.Error("wrong number", "MSETNX", "one", "two", "three")
	})
}

func TestSetx(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SETEX", "foo", "12", "bar")
		c.Do("GET", "foo")
		c.Do("TTL", "foo")
		c.Error("wrong number", "SETEX", "foo")
		c.Error("not an integer", "SETEX", "foo", "noint", "bar")
		c.Error("wrong number", "SETEX", "foo", "12")
		c.Error("wrong number", "SETEX", "foo", "12", "bar", "toomany")
		c.Error("wrong number", "SETEX", "foo", "0")
		c.Error("wrong number", "SETEX", "foo", "-12")

		c.Do("PSETEX", "foo", "12", "bar")
		c.Do("GET", "foo")
		// c.Do("PTTL", "foo") // counts down too quickly to compare
		c.Error("wrong number", "PSETEX", "foo")
		c.Error("not an integer", "PSETEX", "foo", "noint", "bar")
		c.Error("wrong number", "PSETEX", "foo", "12")
		c.Error("wrong number", "PSETEX", "foo", "12", "bar", "toomany")
		c.Error("wrong number", "PSETEX", "foo", "0")
		c.Error("wrong number", "PSETEX", "foo", "-12")
	})
}

func TestGetrange(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "foo", "The quick brown fox jumps over the lazy dog")
		c.Do("GETRANGE", "foo", "0", "100")
		c.Do("GETRANGE", "foo", "0", "0")
		c.Do("GETRANGE", "foo", "0", "-4")
		c.Do("GETRANGE", "foo", "0", "-400")
		c.Do("GETRANGE", "foo", "-4", "-4")
		c.Do("GETRANGE", "foo", "4", "2")
		c.Error("not an integer", "GETRANGE", "foo", "aap", "2")
		c.Error("not an integer", "GETRANGE", "foo", "4", "aap")
		c.Error("wrong number", "GETRANGE", "foo", "4", "2", "aap")
		c.Error("wrong number", "GETRANGE", "foo")
		c.Do("HSET", "aap", "noot", "mies")
		c.Error("wrong kind", "GETRANGE", "aap", "4", "2")
	})
}

func TestStrlen(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "str", "The quick brown fox jumps over the lazy dog")
		c.Do("STRLEN", "str")
		// failure cases
		c.Error("wrong number", "STRLEN")
		c.Error("wrong number", "STRLEN", "str", "bar")
		c.Do("HSET", "hash", "key", "value")
		c.Error("wrong kind", "STRLEN", "hash")
	})
}

func TestSetrange(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "foo", "The quick brown fox jumps over the lazy dog")
		c.Do("SETRANGE", "foo", "0", "aap")
		c.Do("GET", "foo")
		c.Do("SETRANGE", "foo", "10", "noot")
		c.Do("GET", "foo")
		c.Do("SETRANGE", "foo", "40", "overtheedge")
		c.Do("GET", "foo")
		c.Do("SETRANGE", "foo", "400", "oh, hey there")
		c.Do("GET", "foo")
		// Non existing key
		c.Do("SETRANGE", "nosuch", "2", "aap")
		c.Do("GET", "nosuch")

		// Error cases
		c.Error("wrong number", "SETRANGE", "foo")
		c.Error("wrong number", "SETRANGE", "foo", "1")
		c.Error("not an integer", "SETRANGE", "foo", "aap", "bar")
		c.Error("not an integer", "SETRANGE", "foo", "noint", "bar")
		c.Error("out of range", "SETRANGE", "foo", "-1", "bar")
		c.Do("HSET", "aap", "noot", "mies")
		c.Error("wrong kind", "SETRANGE", "aap", "4", "bar")
	})
}

func TestIncrAndFriends(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("INCR", "aap")
		c.Do("INCR", "aap")
		c.Do("INCR", "aap")
		c.Do("GET", "aap")
		c.Do("DECR", "aap")
		c.Do("DECR", "noot")
		c.Do("DECR", "noot")
		c.Do("GET", "noot")
		c.Do("INCRBY", "noot", "100")
		c.Do("INCRBY", "noot", "200")
		c.Do("INCRBY", "noot", "300")
		c.Do("GET", "noot")
		c.Do("DECRBY", "noot", "100")
		c.Do("DECRBY", "noot", "200")
		c.Do("DECRBY", "noot", "300")
		c.Do("DECRBY", "noot", "400")
		c.Do("GET", "noot")
		c.Do("INCRBYFLOAT", "zus", "1.23")
		c.Do("INCRBYFLOAT", "zus", "3.1456")
		c.Do("INCRBYFLOAT", "zus", "987.65432")
		c.Do("GET", "zus")
		c.Do("INCRBYFLOAT", "whole", "300")
		c.Do("INCRBYFLOAT", "whole", "300")
		c.Do("INCRBYFLOAT", "whole", "300")
		c.Do("GET", "whole")
		c.Do("INCRBYFLOAT", "big", "12345e10")
		c.Do("GET", "big")

		// Floats are not ints.
		c.Do("SET", "float", "1.23")
		c.Error("not an integer", "INCR", "float")
		c.Error("not an integer", "INCRBY", "float", "12")
		c.Error("not an integer", "DECR", "float")
		c.Error("not an integer", "DECRBY", "float", "12")
		c.Do("SET", "str", "I'm a string")
		c.Error("not a valid float", "INCRBYFLOAT", "str", "123.5")

		// Error cases
		c.Do("HSET", "mies", "noot", "mies")
		c.Error("wrong kind", "INCR", "mies")
		c.Error("wrong kind", "INCRBY", "mies", "1")
		c.Error("not an integer", "INCRBY", "mies", "foo")
		c.Error("wrong kind", "DECR", "mies")
		c.Error("wrong kind", "DECRBY", "mies", "1")
		c.Error("wrong kind", "INCRBYFLOAT", "mies", "1")
		c.Error("not a valid float", "INCRBYFLOAT", "int", "foo")

		c.Error("wrong number", "INCR", "int", "err")
		c.Error("wrong number", "INCRBY", "int")
		c.Error("wrong number", "DECR", "int", "err")
		c.Error("wrong number", "DECRBY", "int")
		c.Error("wrong number", "INCRBYFLOAT", "int")

		// Rounding
		c.Do("INCRBYFLOAT", "zero", "12.3")
		c.Do("INCRBYFLOAT", "zero", "-13.1")

		// Overflow
		c.Do("SET", "overflow-up", "9223372036854775807")
		c.Error("increment or decrement would overflow", "INCR", "overflow-up")
		c.Error("increment or decrement would overflow", "INCRBY", "overflow-up", "1")
		c.Error("increment or decrement would overflow", "DECRBY", "overflow-up", "-1")

		c.Do("SET", "overflow-down", "-9223372036854775808")
		c.Error("increment or decrement would overflow", "DECR", "overflow-down")
		c.Error("increment or decrement would overflow", "INCRBY", "overflow-down", "-1")
		c.Error("increment or decrement would overflow", "DECRBY", "overflow-down", "1")

		// E
		c.Do("INCRBYFLOAT", "one", "12e12")
		// c.Do("INCRBYFLOAT", "one", "12e34") // FIXME
		c.Error("not a valid float", "INCRBYFLOAT", "one", "12e34.1")
		// c.Do("INCRBYFLOAT", "one", "0x12e12") // FIXME
		// c.Do("INCRBYFLOAT", "one", "012e12") // FIXME
		c.Do("INCRBYFLOAT", "two", "012")
		c.Error("not a valid float", "INCRBYFLOAT", "one", "0b12e12")
	})
}

func TestBitcount(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "str", "The quick brown fox jumps over the lazy dog")
		c.Do("SET", "utf8", "❆❅❄☃")
		c.Do("BITCOUNT", "str")
		c.Do("BITCOUNT", "utf8")
		c.Do("BITCOUNT", "str", "0", "0")
		c.Do("BITCOUNT", "str", "1", "2")
		c.Do("BITCOUNT", "str", "1", "-200")
		c.Do("BITCOUNT", "str", "-2", "-1")
		c.Do("BITCOUNT", "str", "-2", "-12")
		c.Do("BITCOUNT", "utf8", "0", "0")

		c.Do("SETBIT", "A", "10", "1")
		c.Do("BITCOUNT", "A", "0", "100000")
		c.Do("BITCOUNT", "A", "0", "9223372036854775806")
		c.Do("BITCOUNT", "A", "0", "9223372036854775807") // max int64
		c.Error("out of range", "BITCOUNT", "A", "0", "9223372036854775808")

		c.Error("wrong number", "BITCOUNT")
		c.Error("syntax error", "BITCOUNT", "wrong", "arguments")
		c.Error("syntax error", "BITCOUNT", "str", "4", "2", "2", "2", "2")
		c.Error("not an integer", "BITCOUNT", "str", "foo", "2")
		c.Do("HSET", "aap", "noot", "mies")
		c.Error("wrong kind", "BITCOUNT", "aap", "4", "2")
	})
}

func TestBitop(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "a", "foo")
		c.Do("SET", "b", "aap")
		c.Do("SET", "c", "noot")
		c.Do("SET", "d", "mies")
		c.Do("SET", "e", "❆❅❄☃")

		// ANDs
		c.Do("BITOP", "AND", "target", "a", "b", "c", "d")
		c.Do("GET", "target")
		c.Do("BITOP", "AND", "target", "a", "nosuch", "c", "d")
		c.Do("GET", "target")
		c.Do("BITOP", "AND", "utf8", "e", "e")
		c.Do("GET", "utf8")
		c.Do("BITOP", "AND", "utf8", "b", "e")
		c.Do("GET", "utf8")
		// BITOP on only unknown keys:
		c.Do("BITOP", "AND", "bits", "nosuch", "nosucheither")
		c.Do("GET", "bits")

		// ORs
		c.Do("BITOP", "OR", "target", "a", "b", "c", "d")
		c.Do("GET", "target")
		c.Do("BITOP", "OR", "target", "a", "nosuch", "c", "d")
		c.Do("GET", "target")
		c.Do("BITOP", "OR", "utf8", "e", "e")
		c.Do("GET", "utf8")
		c.Do("BITOP", "OR", "utf8", "b", "e")
		c.Do("GET", "utf8")
		// BITOP on only unknown keys:
		c.Do("BITOP", "OR", "bits", "nosuch", "nosucheither")
		c.Do("GET", "bits")
		c.Do("SET", "empty", "")
		// BITOP on empty key
		c.Do("BITOP", "OR", "bits", "empty")
		c.Do("GET", "bits")

		// XORs
		c.Do("BITOP", "XOR", "target", "a", "b", "c", "d")
		c.Do("GET", "target")
		c.Do("BITOP", "XOR", "target", "a", "nosuch", "c", "d")
		c.Do("GET", "target")
		c.Do("BITOP", "XOR", "target", "a")
		c.Do("GET", "target")
		c.Do("BITOP", "XOR", "utf8", "e", "e")
		c.Do("GET", "utf8")
		c.Do("BITOP", "XOR", "utf8", "b", "e")
		c.Do("GET", "utf8")

		// NOTs
		c.Do("BITOP", "NOT", "target", "a")
		c.Do("GET", "target")
		c.Do("BITOP", "NOT", "target", "e")
		c.Do("GET", "target")
		c.Do("BITOP", "NOT", "bits", "nosuch")
		c.Do("GET", "bits")

		c.Error("wrong number", "BITOP", "AND", "utf8")
		c.Error("wrong number", "BITOP", "AND")
		c.Error("single source key", "BITOP", "NOT", "foo", "bar", "baz")
		c.Error("wrong number", "BITOP", "WRONGOP", "key")
		c.Error("wrong number", "BITOP", "WRONGOP")

		c.Do("HSET", "hash", "aap", "noot")
		c.Error("wrong kind", "BITOP", "AND", "t", "hash", "irrelevant")
		c.Error("wrong kind", "BITOP", "OR", "t", "hash", "irrelevant")
		c.Error("wrong kind", "BITOP", "XOR", "t", "hash", "irrelevant")
		c.Error("wrong kind", "BITOP", "NOT", "t", "hash")
	})
}

func TestBitpos(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "a", "\x00\x0f")
		c.Do("SET", "b", "\xf0\xf0")
		c.Do("SET", "c", "\x00\x00\x00\x0f")
		c.Do("SET", "d", "\x00\x00\x00")
		c.Do("SET", "e", "\xff\xff\xff")
		c.Do("SET", "empty", "")

		c.Do("BITPOS", "a", "1")
		c.Do("BITPOS", "a", "0")
		c.Do("BITPOS", "a", "1", "1")
		c.Do("BITPOS", "a", "0", "1")
		c.Do("BITPOS", "a", "1", "1", "2")
		c.Do("BITPOS", "a", "0", "1", "2")
		c.Do("BITPOS", "a", "0", "0", "0")
		c.Do("BITPOS", "a", "0", "0", "-1")
		c.Do("BITPOS", "a", "0", "0", "-2")
		c.Do("BITPOS", "a", "0", "0", "-2")
		c.Do("BITPOS", "a", "0", "0", "-999")
		c.Do("BITPOS", "a", "0", "-1", "-1")
		c.Do("BITPOS", "a", "0", "-2", "-1")
		c.Do("BITPOS", "a", "0", "-2", "-999")
		c.Do("BITPOS", "a", "0", "-999", "-999")
		c.Do("BITPOS", "b", "1")
		c.Do("BITPOS", "b", "0")
		c.Do("BITPOS", "c", "1")
		c.Do("BITPOS", "c", "0")
		c.Do("BITPOS", "d", "1")
		c.Do("BITPOS", "d", "0")
		c.Do("BITPOS", "e", "1")
		c.Do("BITPOS", "e", "0")
		c.Do("BITPOS", "e", "1", "1")
		c.Do("BITPOS", "e", "0", "1")
		c.Do("BITPOS", "e", "1", "1", "2")
		c.Do("BITPOS", "e", "0", "1", "2")
		c.Do("BITPOS", "e", "1", "100", "2")
		c.Do("BITPOS", "e", "0", "100", "2")
		c.Do("BITPOS", "e", "1", "1", "0")
		c.Do("BITPOS", "e", "1", "1", "-1")
		c.Do("BITPOS", "e", "1", "1", "-2")
		c.Do("BITPOS", "e", "1", "1", "-2000")
		c.Do("BITPOS", "e", "0", "0", "0")
		c.Do("BITPOS", "e", "0", "0", "-1")
		c.Do("BITPOS", "e", "0", "1", "2")
		c.Do("BITPOS", "empty", "0")
		c.Do("BITPOS", "empty", "0", "0")
		c.Do("BITPOS", "empty", "0", "0", "0")
		c.Do("BITPOS", "empty", "0", "0", "-1")
		c.Do("BITPOS", "empty", "0", "-1", "-1")
		c.Do("BITPOS", "empty", "1")
		c.Do("BITPOS", "empty", "1", "0")
		c.Do("BITPOS", "empty", "1", "0", "0")
		c.Do("BITPOS", "empty", "1", "0", "-1")
		c.Do("BITPOS", "empty", "1", "-1", "-1")
		c.Do("BITPOS", "nosuch", "0")
		c.Do("BITPOS", "nosuch", "0", "0")
		c.Do("BITPOS", "nosuch", "0", "0", "0")
		c.Do("BITPOS", "nosuch", "1")
		c.Do("BITPOS", "nosuch", "1", "0")
		c.Do("BITPOS", "nosuch", "1", "0", "0")

		c.Do("HSET", "hash", "aap", "noot")
		c.Error("wrong kind", "BITPOS", "hash", "1")
		c.Error("not an integer", "BITPOS", "a", "aap")
	})
}

func TestGetbit(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		for i := 0; i < 100; i++ {
			c.Do("SET", "a", "\x00\x0f")
			c.Do("SET", "e", "\xff\xff\xff")
			c.Do("GETBIT", "nosuch", "1")
			c.Do("GETBIT", "nosuch", "0")

			// Error cases
			c.Do("HSET", "hash", "aap", "noot")
			c.Error("wrong kind", "GETBIT", "hash", "1")
			c.Error("not an integer", "GETBIT", "a", "aap")
			c.Error("wrong number", "GETBIT", "a")
			c.Error("wrong number", "GETBIT", "too", "1", "many")

			c.Do("GETBIT", "a", strconv.Itoa(i))
			c.Do("GETBIT", "e", strconv.Itoa(i))
		}
	})
}

func TestSetbit(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		for i := 0; i < 100; i++ {
			c.Do("SET", "a", "\x00\x0f")
			c.Do("SETBIT", "a", "0", "1")
			c.Do("GET", "a")
			c.Do("SETBIT", "a", "0", "0")
			c.Do("GET", "a")
			c.Do("SETBIT", "a", "13", "0")
			c.Do("GET", "a")
			c.Do("SETBIT", "nosuch", "11111", "1")
			c.Do("GET", "nosuch")

			// Error cases
			c.Do("HSET", "hash", "aap", "noot")
			c.Error("wrong kind", "SETBIT", "hash", "1", "1")
			c.Error("not an integer", "SETBIT", "a", "aap", "0")
			c.Error("not an integer", "SETBIT", "a", "0", "aap")
			c.Error("not an integer", "SETBIT", "a", "-1", "0")
			c.Error("not an integer", "SETBIT", "a", "1", "-1")
			c.Error("not an integer", "SETBIT", "a", "1", "2")
			c.Error("wrong number", "SETBIT", "too", "1", "2", "many")

			c.Do("GETBIT", "a", strconv.Itoa(i))
			c.Do("GETBIT", "e", strconv.Itoa(i))
		}
	})
}

func TestAppend(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "foo", "bar")
		c.Do("APPEND", "foo", "more")
		c.Do("GET", "foo")
		c.Do("APPEND", "nosuch", "more")
		c.Do("GET", "nosuch")

		// Failure cases
		c.Error("wrong number", "APPEND")
		c.Error("wrong number", "APPEND", "foo")
	})
}

func TestMove(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "foo", "bar")
		c.Do("EXPIRE", "foo", "12345")
		c.Do("MOVE", "foo", "2")
		c.Do("GET", "foo")
		c.Do("TTL", "foo")
		c.Do("SELECT", "2")
		c.Do("GET", "foo")
		c.Do("TTL", "foo")

		// Failure cases
		c.Error("wrong number", "MOVE")
		c.Error("wrong number", "MOVE", "foo")
		// c.Do("MOVE", "foo", "noint")
	})
	// hash key
	testRaw(t, func(c *client) {
		c.Do("HSET", "hash", "key", "value")
		c.Do("EXPIRE", "hash", "12345")
		c.Do("MOVE", "hash", "2")
		c.Do("MGET", "hash", "key")
		c.Do("TTL", "hash")
		c.Do("SELECT", "2")
		c.Do("MGET", "hash", "key")
		c.Do("TTL", "hash")
	})
	testRaw(t, func(c *client) {
		c.Do("SET", "foo", "bar")
		// to current DB.
		c.Error("the same", "MOVE", "foo", "0")
	})
}
