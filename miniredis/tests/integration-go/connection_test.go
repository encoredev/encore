package main

import (
	"testing"
)

func TestEcho(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("ECHO", "hello world")
		c.Do("ECHO", "42")
		c.Do("ECHO", "3.1415")
		c.Error("wrong number", "ECHO", "hello", "world")
		c.Error("wrong number", "ECHO")
		c.Error("wrong number", "eChO", "hello", "world")
	})

	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("ECHO", "hi")
		c.Do("EXEC")
	})

	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Error("wrong number", "ECHO")
		c.Error("discarded", "EXEC")
	})
}

func TestPing(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("PING")
		c.Do("PING", "hello world")
		c.Error("wrong number", "PING", "hello", "world")
	})

	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("PING", "hi")
		c.Do("EXEC")
	})

	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("PING", "hi again")
		c.Do("EXEC")
	})
}

func TestSelect(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("SET", "foo", "bar")
		c.Do("GET", "foo")
		c.Do("SELECT", "2")
		c.Do("GET", "foo")
		c.Do("SET", "foo", "bar2")
		c.Do("GET", "foo")

		c.Error("wrong number", "SELECT")
		c.Error("out of range", "SELECT", "-1")
		c.Error("not an integer", "SELECT", "aap")
		c.Error("wrong number", "SELECT", "1", "2")
	})

	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("SET", "foo", "bar")
		c.Do("GET", "foo")
		c.Do("SELECT", "2")
		c.Do("GET", "foo")
		c.Do("SET", "foo", "bar2")
		c.Do("GET", "foo")
		c.Do("EXEC")
		c.Do("GET", "foo")
	})

	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("SELECT", "-1")
		c.Do("EXEC")
	})
}

func TestAuth(t *testing.T) {
	skip(t)
	testAuth(t,
		"supersecret",
		func(c *client) {
			c.Error("Authentication required", "PING")
			c.Error("Authentication required", "SET", "foo", "bar")
			c.Error("wrong number", "SET")
			c.Error("Authentication required", "SET", "foo", "bar", "baz")
			c.Error("Authentication required", "GET", "foo")
			c.Error("wrong number", "AUTH")
			c.Error("invalid", "AUTH", "nosecret")
			c.Error("invalid", "AUTH", "nosecret", "bar")
			c.Error("syntax error", "AUTH", "nosecret", "bar", "bar")
			c.Do("AUTH", "supersecret")
			c.Do("SET", "foo", "bar")
			c.Do("GET", "foo")
		},
	)

	testUserAuth(t,
		map[string]string{
			"agent1": "supersecret",
			"agent2": "dragon",
		},
		func(c *client) {
			c.Error("Authentication required", "PING")
			c.Error("Authentication required", "SET", "foo", "bar")
			c.Error("wrong number", "SET")
			c.Error("Authentication required", "SET", "foo", "bar", "baz")
			c.Error("Authentication required", "GET", "foo")
			c.Error("wrong number", "AUTH")
			c.Error("invalid", "AUTH", "nosecret")
			c.Error("invalid", "AUTH", "agent100", "supersecret")
			c.Error("syntax error", "AUTH", "agent100", "supersecret", "supersecret")
			c.Error("invalid", "AUTH", "agent1", "bzzzt")
			c.Do("AUTH", "agent1", "supersecret")
			c.Do("SET", "foo", "bar")
			c.Do("GET", "foo")

			// go back to invalid user
			c.Error("invalid", "AUTH", "agent100", "supersecret")
			c.Do("GET", "foo") // still agent1
		},
	)

	testRaw(t, func(c *client) {
		c.Error("wrong number", "AUTH")
		c.Error("without any", "AUTH", "foo")
		c.Error("invalid", "AUTH", "foo", "bar")
		c.Error("syntax error", "AUTH", "foo", "bar", "bar")
	})

	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("AUTH", "apassword")
		c.Do("EXEC")
	})
}

func TestHello(t *testing.T) {
	skip(t)
	testRaw(t,
		func(c *client) {
			c.Do("SADD", "s", "aap") // sets have resp3 specific code

			c.DoLoosely("HELLO", "3")
			c.Do("SMEMBERS", "s")

			c.DoLoosely("HELLO", "2")
			c.Do("SMEMBERS", "s")

			c.Error("not an integer", "HELLO", "twoandahalf")

			c.DoLoosely("HELLO", "3", "AUTH", "default", "foo")
			c.DoLoosely("HELLO", "3", "AUTH", "default", "foo", "SETNAME", "foo")
			c.DoLoosely("HELLO", "3", "SETNAME", "foo")

			// errors
			c.Error("Syntax error", "HELLO", "3", "default", "foo")
			c.Error("not an integer", "HELLO", "three", "AUTH", "default", "foo")
			c.Error("Syntax error", "HELLO", "3", "AUTH", "default")
			c.Error("unsupported", "HELLO", "-1", "foo")
			c.Error("unsupported", "HELLO", "0", "foo")
			c.Error("unsupported", "HELLO", "1", "foo")
			c.Error("unsupported", "HELLO", "4", "foo")
			c.Error("Syntax error", "HELLO", "3", "default", "foo", "SETNAME")
			c.Error("Syntax error", "HELLO", "3", "SETNAME")

		},
	)

	testAuth(t,
		"secret",
		func(c *client) {
			c.Error("Authentication required", "SADD", "s", "aap") // sets have resp3 specific code

			c.Error("invalid", "HELLO", "3", "AUTH", "default", "foo")
			c.Error("invalid", "HELLO", "3", "AUTH", "wrong", "secret")
			c.DoLoosely("HELLO", "3", "AUTH", "default", "secret")
			c.Do("SMEMBERS", "s")
			c.DoLoosely("HELLO", "3", "AUTH", "default", "secret") // again!
			c.Do("SMEMBERS", "s")
			c.DoLoosely("HELLO", "2", "AUTH", "default", "secret") // again!
			c.Do("SMEMBERS", "s")

			c.DoLoosely("HELLO", "3", "AUTH", "default", "wrong")
			c.Do("SMEMBERS", "s")
		},
	)

	testUserAuth(t,
		map[string]string{
			"sesame": "open",
		},
		func(c *client) {
			c.Error("Authentication required", "SADD", "s", "aap") // sets have resp3 specific code

			c.Error("invalid", "HELLO", "3", "AUTH", "foo", "bar")
			c.Error("invalid", "HELLO", "3", "AUTH", "sesame", "close")
			c.Error("Authentication required", "SMEMBERS", "s")
			c.DoLoosely("HELLO", "3", "AUTH", "sesame", "open123")
			c.Error("Authentication required", "SMEMBERS", "s")
		},
	)
}
