package main

import (
	"sync"
	"testing"
	"time"
)

func TestStream(t *testing.T) {
	skip(t)
	t.Run("XADD", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("XADD",
				"planets",
				"0-1",
				"name", "Mercury",
			)
			c.DoLoosely("XADD",
				"planets",
				"*",
				"name", "Venus",
			)
			c.Do("XADD",
				"planets",
				"18446744073709551000-0",
				"name", "Earth",
			)
			c.Do("XADD",
				"planets",
				"18446744073709551000-*",
				"name", "Pluto",
			)
			c.Do("XADD",
				"reallynosuchkey",
				"NOMKSTREAM",
				"*",
				"name", "Earth",
			)
			c.Error("ID specified", "XADD",
				"planets",
				"18446744073709551000-0", // <-- duplicate
				"name", "Earth",
			)
			c.Do("XLEN", "planets")
			c.Do("RENAME", "planets", "planets2")
			c.Do("DEL", "planets2")
			c.Do("XLEN", "planets")

			// error cases
			c.Error("wrong number", "XADD",
				"planets",
				"1000",
				"name", "Mercury",
				"ignored", // <-- not an even number of keys
			)
			c.Error("ID specified", "XADD",
				"newplanets",
				"0", // <-- invalid key
				"foo", "bar",
			)
			c.Error("wrong number", "XADD", "newplanets", "123-123") // no args
			c.Error("stream ID", "XADD", "newplanets", "123-bar", "foo", "bar")
			c.Error("stream ID", "XADD", "newplanets", "bar-123", "foo", "bar")
			c.Error("stream ID", "XADD", "newplanets", "123-123-123", "foo", "bar")
			c.Do("SET", "str", "I am a string")
			// c.Do("XADD", "str", "1000", "foo", "bar")
			// c.Do("XADD", "str", "invalid-key", "foo", "bar")

			c.Error("wrong number", "XADD", "planets")
			c.Error("wrong number", "XADD")
		})

		testRaw(t, func(c *client) {
			c.Do("XADD", "planets", "MAXLEN", "4", "456-1", "name", "Mercury")
			c.Do("XADD", "planets", "MAXLEN", "4", "456-2", "name", "Mercury")
			c.Do("XADD", "planets", "MAXLEN", "4", "456-3", "name", "Mercury")
			c.Do("XADD", "planets", "MAXLEN", "4", "456-4", "name", "Mercury")
			c.Do("XADD", "planets", "MAXLEN", "4", "456-5", "name", "Mercury")
			c.Do("XADD", "planets", "MAXLEN", "4", "456-6", "name", "Mercury")
			c.Do("XLEN", "planets")
			c.Do("XADD", "planets", "MAXLEN", "~", "4", "456-7", "name", "Mercury")

			c.Error("not an integer", "XADD", "planets", "MAXLEN", "!", "4", "*", "name", "Mercury")
			c.Error("not an integer", "XADD", "planets", "MAXLEN", " ~", "4", "*", "name", "Mercury")
			c.Error("MAXLEN argument", "XADD", "planets", "MAXLEN", "-4", "*", "name", "Mercury")
			c.Error("not an integer", "XADD", "planets", "MAXLEN", "", "*", "name", "Mercury")
			c.Error("not an integer", "XADD", "planets", "MAXLEN", "!", "four", "*", "name", "Mercury")
			c.Error("not an integer", "XADD", "planets", "MAXLEN", "~", "four")
			c.Error("wrong number", "XADD", "planets", "MAXLEN", "~")
			c.Error("wrong number", "XADD", "planets", "MAXLEN")

			c.Do("XADD", "planets", "MAXLEN", "0", "456-8", "name", "Mercury")
			c.Do("XLEN", "planets")

			c.Do("SET", "str", "I am a string")
			c.Error("not an integer", "XADD", "str", "MAXLEN", "four", "*", "foo", "bar")
		})

		testRaw(t, func(c *client) {
			c.Do("XADD", "planets", "MINID", "450", "450-0", "name", "Venus")
			c.Do("XADD", "planets", "MINID", "450", "450-1", "name", "Venus")
			c.Do("XADD", "planets", "MINID", "450", "456-1", "name", "Mercury")
			c.Do("XADD", "planets", "MINID", "450", "456-2", "name", "Mercury")
			c.Do("XADD", "planets", "MINID", "450", "456-3", "name", "Mercury")
			c.Do("XADD", "planets", "MINID", "450", "456-4", "name", "Mercury")
			c.Do("XADD", "planets", "MINID", "450", "456-5", "name", "Mercury")
			c.Do("XADD", "planets", "MINID", "450", "456-6", "name", "Mercury")
			c.Do("XADD", "planets", "MINID", "~", "450", "456-7", "name", "Mercury")
			c.Do("XLEN", "planets")

			c.Error("equal or smaller than the target", "XADD", "planets", "MINID", "450", "449-0", "name", "Earth")
			c.Error("equal or smaller than the target", "XADD", "planets", "MINID", "450", "450", "name", "Earth")
			c.Error("wrong number", "XADD", "planets", "MINID", "~")
			c.Error("wrong number", "XADD", "planets", "MINID")
			c.Error("wrong number", "XADD", "planets", "MINID", "100")

			c.Do("SET", "str", "I am a string")
			c.Error("key holding the wrong kind of value", "XADD", "str", "MINID", "400", "*", "foo", "bar")
		})
	})

	t.Run("transactions", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("MULTI")
			c.Do("XADD", "planets", "0-1", "name", "Mercury")
			c.Do("EXEC")

			c.Do("MULTI")
			c.Error("wrong number", "XADD", "newplanets", "123-123") // no args
			c.Error("discarded", "EXEC")

			c.Do("MULTI")
			c.Do("XADD", "planets", "foo-bar", "name", "Mercury")
			c.Do("EXEC")

			c.Do("MULTI")
			c.Do("XADD", "planets", "MAXLEN", "four", "*", "name", "Mercury")
			c.Do("EXEC")

			c.Do("MULTI")
			c.Do("XADD", "reallynosuchkey", "NOMKSTREAM", "MAXLEN", "four", "*", "name", "Mercury")
			c.Do("EXEC")
		})
	})

	t.Run("XDEL", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("XDEL", "newplanets", "123-123")
			c.Do("XADD", "newplanets", "123-123", "foo", "bar")
			c.Do("XADD", "newplanets", "123-124", "baz", "bak")
			c.Do("XADD", "newplanets", "123-125", "bal", "bag")
			c.Do("XDEL", "newplanets", "123-123", "123-125", "123-123")
			c.Do("XREAD", "STREAMS", "newplanets", "0")
			c.Do("XDEL", "newplanets", "123-123")
			c.Do("XREAD", "STREAMS", "newplanets", "0")
			c.Do("XDEL", "notexisting", "123-123")
			c.Do("XREAD", "STREAMS", "newplanets", "0")

			c.Do("XADD", "gaps", "400-400", "foo", "bar")
			c.Do("XADD", "gaps", "400-600", "foo", "bar")
			c.Do("XDEL", "gaps", "400-500")
			c.Do("XREAD", "STREAMS", "newplanets", "0")

			// errors
			c.Do("XADD", "existing", "123-123", "foo", "bar")
			c.Error("wrong number", "XDEL")             // no key
			c.Error("wrong number", "XDEL", "existing") // no id
			c.Error("Invalid stream ID", "XDEL", "existing", "aa-bb")
			c.Do("XDEL", "notexisting", "aa-bb") // invalid id

			c.Do("MULTI")
			c.Do("XDEL", "existing", "aa-bb")
			c.Do("EXEC")
		})
	})

	t.Run("FLUSHALL", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("XADD", "planets", "0-1", "name", "Mercury")
			c.Do("XGROUP", "CREATE", "planets", "universe", "$")
			c.Do("FLUSHALL")
			c.Do("XREAD", "STREAMS", "planets", "0")
			c.Error("consumer group", "XREADGROUP", "GROUP", "universe", "alice", "STREAMS", "planets", ">")
		})
	})

	t.Run("XINFO", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("XADD", "planets", "0-1", "name", "Mercury")
			// c.DoLoosely("XINFO", "STREAM", "planets")

			c.Error("unknown subcommand", "XINFO", "STREAMMM")
			c.Error("no such key", "XINFO", "STREAM", "foo")
			c.Error("wrong number", "XINFO")
			c.Do("SET", "scalar", "foo")
			c.Error("wrong kind", "XINFO", "STREAM", "scalar")

			c.Error("no such key", "XINFO", "GROUPS", "foo")
			c.Do("XINFO", "GROUPS", "planets")

			c.Error("no such key", "XINFO", "CONSUMERS", "foo", "bar")
		})
	})

	t.Run("XREAD", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("XADD",
				"ordplanets",
				"0-1",
				"name", "Mercury",
				"greek-god", "Hermes",
			)
			c.Do("XADD",
				"ordplanets",
				"1-0",
				"name", "Venus",
				"greek-god", "Aphrodite",
			)
			c.Do("XADD",
				"ordplanets",
				"2-1",
				"greek-god", "",
				"name", "Earth",
			)
			c.Do("XADD",
				"ordplanets",
				"3-0",
				"name", "Mars",
				"greek-god", "Ares",
			)
			c.Do("XADD",
				"ordplanets",
				"4-1",
				"greek-god", "Dias",
				"name", "Jupiter",
			)
			c.Do("XADD", "ordplanets2", "0-1", "name", "Mercury", "greek-god", "Hermes", "idx", "1")
			c.Do("XADD", "ordplanets2", "1-0", "name", "Venus", "greek-god", "Aphrodite", "idx", "2")
			c.Do("XADD", "ordplanets2", "2-1", "name", "Earth", "greek-god", "", "idx", "3")
			c.Do("XADD", "ordplanets2", "3-0", "greek-god", "Ares", "name", "Mars", "idx", "4")
			c.Do("XADD", "ordplanets2", "4-1", "name", "Jupiter", "greek-god", "Dias", "idx", "5")

			c.Do("XREAD", "STREAMS", "ordplanets", "0")
			c.Do("XREAD", "STREAMS", "ordplanets", "2")
			c.Do("XREAD", "STREAMS", "ordplanets", "ordplanets2", "0", "0")
			c.Do("XREAD", "STREAMS", "ordplanets", "ordplanets2", "2", "0")
			c.Do("XREAD", "STREAMS", "ordplanets", "ordplanets2", "0", "2")
			c.Do("XREAD", "STREAMS", "ordplanets", "ordplanets2", "1", "3")
			c.Do("XREAD", "STREAMS", "ordplanets", "ordplanets2", "0", "999")
			c.Do("XREAD", "COUNT", "1", "STREAMS", "ordplanets", "ordplanets2", "0", "0")

			// failure cases
			c.Error("wrong number", "XREAD")
			c.Error("wrong number", "XREAD", "STREAMS")
			c.Error("wrong number", "XREAD", "STREAMS", "foo")
			c.Do("XREAD", "STREAMS", "foo", "0")
			c.Error("wrong number", "XREAD", "STREAMS", "ordplanets")
			c.Error("Unbalanced 'xread'", "XREAD", "STREAMS", "ordplanets", "foo", "0")
			c.Error("wrong number", "XREAD", "COUNT")
			c.Error("wrong number", "XREAD", "COUNT", "notint")
			c.Error("wrong number", "XREAD", "COUNT", "10") // No streams
			c.Error("stream ID", "XREAD", "STREAMS", "foo", "notint")
		})

		testRaw2(t, func(c, c2 *client) {
			c.Do("XADD", "pl", "55-88", "name", "Mercury")
			// something is available: doesn't block
			c.Do("XREAD", "BLOCK", "10", "STREAMS", "pl", "0")
			c.Do("XREAD", "BLOCK", "0", "STREAMS", "pl", "0")

			// blocks
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				c.Do("XREAD", "BLOCK", "1000", "STREAMS", "pl", "60")
				wg.Done()
			}()
			time.Sleep(10 * time.Millisecond)
			c2.Do("XADD", "pl", "60-1", "name", "Mercury")
			wg.Wait()

			// timeout
			c.Do("XREAD", "BLOCK", "10", "STREAMS", "pl", "70")

			c.Error("not an int", "XREAD", "BLOCK", "foo", "STREAMS", "pl", "0")
			c.Error("negative", "XREAD", "BLOCK", "-12", "STREAMS", "pl", "0")
		})

		// special '$' ID
		testRaw2(t, func(c, c2 *client) {
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				time.Sleep(10 * time.Millisecond)
				c2.Do("XADD", "pl", "60-1", "name", "Mercury")
				wg.Done()
			}()
			wg.Wait()
			c.Do("XREAD", "BLOCK", "1000", "STREAMS", "pl", "$")
		})

		// special '$' ID on non-existing stream
		testRaw2(t, func(c, c2 *client) {
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				time.Sleep(10 * time.Millisecond)
				c2.Do("XADD", "pl", "60-1", "nosuch", "Mercury")
				wg.Done()
			}()
			wg.Wait()
			c.Do("XREAD", "BLOCK", "1000", "STREAMS", "nosuch", "$")
		})
	})
}

func TestStreamRange(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("XADD",
			"ordplanets",
			"0-1",
			"name", "Mercury",
			"greek-god", "Hermes",
		)
		c.Do("XADD",
			"ordplanets",
			"1-0",
			"name", "Venus",
			"greek-god", "Aphrodite",
		)
		c.Do("XADD",
			"ordplanets",
			"2-1",
			"greek-god", "",
			"name", "Earth",
		)
		c.Do("XADD",
			"ordplanets",
			"3-0",
			"name", "Mars",
			"greek-god", "Ares",
		)
		c.Do("XADD",
			"ordplanets",
			"4-1",
			"greek-god", "Dias",
			"name", "Jupiter",
		)
		c.Do("XRANGE", "ordplanets", "-", "+")
		c.Do("XRANGE", "ordplanets", "+", "-")
		c.Do("XRANGE", "ordplanets", "-", "99")
		c.Do("XRANGE", "ordplanets", "0", "4")
		c.Do("XRANGE", "ordplanets", "(0", "4")
		c.Do("XRANGE", "ordplanets", "0", "(4")
		c.Do("XRANGE", "ordplanets", "(0", "(4")
		c.Do("XRANGE", "ordplanets", "2", "2")
		c.Do("XRANGE", "ordplanets", "2-0", "2-1")
		c.Do("XRANGE", "ordplanets", "2-1", "2-1")
		c.Do("XRANGE", "ordplanets", "2-1", "2-2")
		c.Do("XRANGE", "ordplanets", "0", "1-0")
		c.Do("XRANGE", "ordplanets", "0", "1-99")
		c.Do("XRANGE", "ordplanets", "0", "2", "COUNT", "1")
		c.Do("XRANGE", "ordplanets", "1-42", "3-42", "COUNT", "1")

		c.Do("XREVRANGE", "ordplanets", "+", "-")
		c.Do("XREVRANGE", "ordplanets", "-", "+")
		c.Do("XREVRANGE", "ordplanets", "4", "0")
		c.Do("XREVRANGE", "ordplanets", "(4", "0")
		c.Do("XREVRANGE", "ordplanets", "4", "(0")
		c.Do("XREVRANGE", "ordplanets", "(4", "(0")
		c.Do("XREVRANGE", "ordplanets", "2", "2")
		c.Do("XREVRANGE", "ordplanets", "2-1", "2-0")
		c.Do("XREVRANGE", "ordplanets", "2-1", "2-1")
		c.Do("XREVRANGE", "ordplanets", "2-2", "2-1")
		c.Do("XREVRANGE", "ordplanets", "1-0", "0")
		c.Do("XREVRANGE", "ordplanets", "3-42", "1-0", "COUNT", "2")
		c.Do("DEL", "ordplanets")

		// failure cases
		c.Error("wrong number", "XRANGE")
		c.Error("wrong number", "XRANGE", "foo")
		c.Error("wrong number", "XRANGE", "foo", "1")
		c.Error("syntax error", "XRANGE", "foo", "2", "3", "toomany")
		c.Error("not an integer", "XRANGE", "foo", "2", "3", "COUNT", "noint")
		c.Error("syntax error", "XRANGE", "foo", "2", "3", "COUNT", "1", "toomany")
		c.Error("stream ID", "XRANGE", "foo", "-", "noint")
		c.Error("stream ID", "XRANGE", "foo", "(-", "+")
		c.Error("stream ID", "XRANGE", "foo", "-", "(+")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "XRANGE", "str", "-", "+")
	})

	testRaw(t, func(c *client) {
		c.Do("MULTI")
		c.Do("XADD",
			"ordplanets",
			"0-1",
			"name", "Mercury",
			"greek-god", "Hermes",
		)
		c.Do("XLEN", "ordplanets")
		c.Do("XRANGE", "ordplanets", "+", "-")
		c.Do("XRANGE", "ordplanets", "+", "-", "COUNT", "FOOBAR")
		c.Do("EXEC")
		c.Do("XLEN", "ordplanets")

		c.Do("MULTI")
		c.Do("XRANGE", "ordplanets", "+", "foo")
		c.Do("EXEC")

		c.Do("MULTI")
		c.Error("wrong number", "XRANGE", "ordplanets", "+")
		c.Error("discarded", "EXEC")

		c.Do("MULTI")
		c.Do("XADD", "ordplanets", "123123-123", "name", "Mercury")
		c.Do("XDEL", "ordplanets", "123123-123")
		c.Do("XADD", "ordplanets", "invalid", "name", "Mercury")
		c.Do("EXEC")
		c.Do("XLEN", "ordplanets")
	})
}

func TestStreamGroup(t *testing.T) {
	skip(t)
	t.Run("XGROUP", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Error("to exist", "XGROUP", "CREATE", "planets", "processing", "$")
			c.Do("XADD", "planets", "123-500", "foo", "bar")
			c.Do("XGROUP", "CREATE", "planets", "processing", "$")
			c.DoLoosely("XINFO", "GROUPS", "planets") // lag is wrong
			c.Error("already exist", "XGROUP", "CREATE", "planets", "processing", "$")
			c.Error("to exist", "XGROUP", "DESTROY", "foo", "bar")
			c.Do("XGROUP", "DESTROY", "planets", "bar")
			c.Error("No such consumer group", "XGROUP", "DELCONSUMER", "planets", "foo", "bar")
			c.Do("XGROUP", "CREATECONSUMER", "planets", "processing", "alice")
			c.DoLoosely("XINFO", "GROUPS", "planets") // lag is wrong
			c.Do("XGROUP", "DELCONSUMER", "planets", "processing", "foo")
			c.Do("XGROUP", "DELCONSUMER", "planets", "processing", "alice")
			c.Do("XINFO", "CONSUMERS", "planets", "processing")
			c.Do("XGROUP", "DESTROY", "planets", "processing")
			c.Do("XINFO", "GROUPS", "planets")
			c.Error("wrong number of arguments", "XGROUP")
			c.Error("unknown subcommand 'foo'", "XGROUP", "foo")
		})
	})

	t.Run("XREADGROUP", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("XGROUP", "CREATE", "planets", "processing", "$", "MKSTREAM")
			// succNoResultCheck("XINFO", "STREAM", "planets"),
			c.Do("XADD", "planets", "42-1", "name", "Mercury")
			c.Do("XADD", "planets", "42-2", "name", "Neptune")
			c.Do("XLEN", "planets")
			c.Do("XREADGROUP", "GROUP", "processing", "alice", "STREAMS", "planets", ">")
			c.Do("XREADGROUP", "GROUP", "processing", "alice", "COUNT", "1", "STREAMS", "planets", ">")
			c.Do("XREADGROUP", "GROUP", "processing", "alice", "COUNT", "999", "STREAMS", "planets", ">")
			c.Do("XREADGROUP", "GROUP", "processing", "alice", "COUNT", "0", "STREAMS", "planets", ">")
			c.Do("XREADGROUP", "GROUP", "processing", "alice", "COUNT", "-1", "STREAMS", "planets", ">")
			c.Do("XACK", "planets", "processing", "42-1")
			c.Do("XDEL", "planets", "42-1")
			c.Do("XGROUP", "CREATE", "planets", "newcons", "$", "MKSTREAM")

			c.Do("XREADGROUP", "GROUP", "processing", "bob", "STREAMS", "planets", ">")
			c.Do("XADD", "planets", "42-3", "name", "Venus")
			c.Do("XREADGROUP", "GROUP", "processing", "bob", "STREAMS", "planets", "42-1")
			c.Do("XREADGROUP", "GROUP", "processing", "bob", "STREAMS", "planets", "42-9")
			c.Error("stream ID", "XREADGROUP", "GROUP", "processing", "bob", "STREAMS", "planets", "foo")

			// NOACK
			{
				c.Do("XGROUP", "CREATE", "colors", "pr", "$", "MKSTREAM")
				c.Do("XADD", "colors", "42-2", "name", "Green")
				c.Do("XREADGROUP", "GROUP", "pr", "alice", "NOACK", "STREAMS", "colors", ">")
				c.Do("XREADGROUP", "GROUP", "pr", "alice", "NOACK", "STREAMS", "colors", "0")
				c.Do("XACK", "colors", "p", "42-2")
			}

			// errors
			c.Error("wrong number", "XREADGROUP")
			c.Error("wrong number", "XREADGROUP", "GROUP")
			c.Error("wrong number", "XREADGROUP", "foo")
			c.Error("wrong number", "XREADGROUP", "GROUP", "foo")
			c.Error("wrong number", "XREADGROUP", "GROUP", "foo", "bar")
			c.Error("wrong number", "XREADGROUP", "GROUP", "foo", "bar", "ZTREAMZ")
			c.Error("wrong number", "XREADGROUP", "GROUP", "foo", "bar", "STREAMS", "foo")
			c.Error("Unbalanced", "XREADGROUP", "GROUP", "foo", "bar", "STREAMS", "foo", "bar", ">")
			c.Error("syntax error", "XREADGROUP", "_____", "foo", "bar", "STREAMS", "foo", ">")
			c.Error("consumer group", "XREADGROUP", "GROUP", "nosuch", "alice", "STREAMS", "planets", ">")
			c.Error("consumer group", "XREADGROUP", "GROUP", "processing", "alice", "STREAMS", "nosuchplanets", ">")
			c.Do("SET", "scalar", "bar")
			c.Error("wrong kind", "XGROUP", "CREATE", "scalar", "processing", "$", "MKSTREAM")
			c.Error("BUSYGROUP", "XGROUP", "CREATE", "planets", "processing", "$", "MKSTREAM")
		})

		testRaw2(t, func(c, c2 *client) {
			c.Do("XGROUP", "CREATE", "pl", "processing", "$", "MKSTREAM")
			c.Do("XADD", "pl", "55-88", "name", "Mercury")
			// something is available: doesn't block
			c.Do("XREADGROUP", "GROUP", "processing", "foo", "BLOCK", "10", "STREAMS", "pl", ">")
			// c.Do("XREADGROUP", "GROUP", "processing", "foo", "BLOCK", "0", "STREAMS", "pl", ">")

			// blocks
			{
				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					c.Do("XREADGROUP", "GROUP", "processing", "foo", "BLOCK", "999999", "STREAMS", "pl", ">")
					wg.Done()
				}()
				time.Sleep(50 * time.Millisecond)
				c2.Do("XADD", "pl", "60-1", "name", "Mercury")
				wg.Wait()
			}

			// timeout
			{
				c.Do("XREADGROUP", "GROUP", "processing", "foo", "BLOCK", "10", "STREAMS", "pl", ">")
			}

			// block is ignored if id isn't ">"
			{
				c.Do("XREADGROUP", "GROUP", "processing", "foo", "BLOCK", "9999999999", "STREAMS", "pl", "8")
			}

			// block is ignored if _any_ id isn't ">"
			{
				c.Do("XGROUP", "CREATE", "pl2", "processing", "$", "MKSTREAM")
				c.Do("XREADGROUP", "GROUP", "processing", "foo", "BLOCK", "9999999999", "STREAMS", "pl", "pl2", "8", ">")
			}

			c.Error("not an int", "XREADGROUP", "GROUP", "foo", "bar", "BLOCK", "foo", "STREAMS", "foo", ">")
			c.Error("No such", "XREADGROUP", "GROUP", "foo", "bar", "BLOCK", "999999", "STREAMS", "pl", "invalid")
			c.Error("negative", "XREADGROUP", "GROUP", "foo", "bar", "BLOCK", "-1", "STREAMS", "foo", ">")
		})
	})

	t.Run("XACK", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("XGROUP", "CREATE", "planets", "processing", "$", "MKSTREAM")
			c.Do("XADD", "planets", "4000-1", "name", "Mercury")
			c.Do("XADD", "planets", "4000-2", "name", "Venus")
			c.Do("XADD", "planets", "4000-3", "name", "not Pluto")
			c.Do("XADD", "planets", "4000-4", "name", "Mars")
			c.Do("XREADGROUP", "GROUP", "processing", "alice", "COUNT", "1", "STREAMS", "planets", ">")
			c.Do("XACK", "planets", "processing", "4000-2", "4000-3")
			c.Do("XACK", "planets", "processing", "4000-4")
			c.Do("XACK", "planets", "processing", "2000-1")

			c.Do("XACK", "nosuch", "processing", "0-1")
			c.Do("XACK", "planets", "nosuch", "0-1")

			// error cases
			c.Error("wrong number", "XACK")
			c.Error("wrong number", "XACK", "planets")
			c.Error("wrong number", "XACK", "planets", "processing")
			c.Error("Invalid stream", "XACK", "planets", "processing", "invalid")
			c.Do("SET", "scalar", "bar")
			c.Error("wrong kind", "XACK", "scalar", "processing", "123-456")
		})
	})

	t.Run("XPENDING", func(t *testing.T) {
		// summary mode
		testRaw(t, func(c *client) {
			c.Do("XGROUP", "CREATE", "planets", "processing", "$", "MKSTREAM")
			c.Do("XADD", "planets", "4000-1", "name", "Mercury")
			c.Do("XADD", "planets", "4000-2", "name", "Venus")
			c.Do("XADD", "planets", "4000-3", "name", "not Pluto")
			c.Do("XADD", "planets", "4000-4", "name", "Mars")
			c.Do("XREADGROUP", "GROUP", "processing", "alice", "STREAMS", "planets", ">")
			c.Do("XPENDING", "planets", "processing")
			c.Do("XACK", "planets", "processing", "4000-4")
			c.Do("XPENDING", "planets", "processing")
			c.Do("XACK", "planets", "processing", "4000-1")
			c.Do("XACK", "planets", "processing", "4000-2")
			c.Do("XACK", "planets", "processing", "4000-3")
			c.Do("XPENDING", "planets", "processing")

			// more consumers
			c.Do("XADD", "planets", "4000-5", "name", "Earth")
			c.Do("XADD", "planets", "4000-6", "name", "Neptune")
			c.Do("XREADGROUP", "GROUP", "processing", "alice", "COUNT", "1", "STREAMS", "planets", ">")
			c.Do("XREADGROUP", "GROUP", "processing", "bob", "COUNT", "1", "STREAMS", "planets", ">")
			c.Do("XPENDING", "planets", "processing")

			// no entries doesn't show up in pending
			c.Do("XREADGROUP", "GROUP", "processing", "eve", "COUNT", "1", "STREAMS", "planets", ">")
			c.Do("XPENDING", "planets", "processing")

			c.Do("XGROUP", "DELCONSUMER", "planets", "processing", "alice")
			c.Do("XPENDING", "planets", "processing")

			c.Do("XGROUP", "CREATE", "empty", "empty", "$", "MKSTREAM")
			c.Do("XPENDING", "empty", "empty", "-", "+", "999")

			c.Error("consumer group", "XPENDING", "foo", "processing")
			c.Error("consumer group", "XPENDING", "planets", "foo")

			// error cases
			c.Error("wrong number", "XPENDING")
			c.Error("wrong number", "XPENDING", "planets")
			c.Error("syntax", "XPENDING", "planets", "processing", "too many")
			c.Error("syntax", "XPENDING", "planets", "processing", "IDLE", "10")
		})

		// full mode
		testRaw(t, func(c *client) {
			c.Do("XGROUP", "CREATE", "planets", "processing", "$", "MKSTREAM")
			c.Do("XADD", "planets", "4000-1", "name", "Mercury")
			c.Do("XADD", "planets", "4000-2", "name", "Venus")
			c.Do("XADD", "planets", "4000-3", "name", "not Pluto")
			c.Do("XADD", "planets", "4000-4", "name", "Mars")
			c.Do("XREADGROUP", "GROUP", "processing", "alice", "STREAMS", "planets", ">")

			c.DoLoosely("XPENDING", "planets", "processing", "-", "+", "999")
			c.DoLoosely("XPENDING", "planets", "processing", "4000-2", "+", "999")
			c.DoLoosely("XPENDING", "planets", "processing", "-", "4000-3", "999")
			c.DoLoosely("XPENDING", "planets", "processing", "-", "+", "1")
			c.DoLoosely("XPENDING", "planets", "processing", "-", "+", "0")
			c.DoLoosely("XPENDING", "planets", "processing", "-", "+", "-1")
			c.DoLoosely("XPENDING", "planets", "processing", "IDLE", "10", "-", "+", "999")

			c.Do("XADD", "planets", "4000-5", "name", "Earth")
			c.Do("XREADGROUP", "GROUP", "processing", "bob", "STREAMS", "planets", ">")
			c.DoLoosely("XPENDING", "planets", "processing", "-", "+", "999")
			c.DoLoosely("XPENDING", "planets", "processing", "-", "+", "999", "bob")
			c.DoLoosely("XPENDING", "planets", "processing", "-", "+", "999", "eve")
			c.DoLoosely("XPENDING", "planets", "processing", "IDLE", "10", "-", "+", "999", "eve")

			// update delivery counts (which we can't test thanks to the time field)
			c.Do("XREADGROUP", "GROUP", "processing", "bob", "STREAMS", "planets", "99")
			c.DoLoosely("XPENDING", "planets", "processing", "-", "+", "999", "bob")

			c.Error("Invalid", "XPENDING", "planets", "processing", "foo", "+", "999")
			c.Error("Invalid", "XPENDING", "planets", "processing", "-", "foo", "999")
			c.Error("not an integer", "XPENDING", "planets", "processing", "-", "+", "foo")
			c.Error("not an integer", "XPENDING", "planets", "processing", "IDLE", "abc", "-", "+", "999")
		})
	})

	t.Run("XAUTOCLAIM", func(t *testing.T) {
		// justid mode
		testRaw(t, func(c *client) {
			c.Do("XGROUP", "CREATE", "colors", "pr", "$", "MKSTREAM")
			c.Do("XAUTOCLAIM", "colors", "pr", "alice", "0", "0", "JUSTID")
			c.Do("XADD", "colors", "42-2", "name", "Green")
			c.Do("XADD", "colors", "42-3", "name", "Blue")
			c.Do("XREADGROUP", "GROUP", "pr", "alice", "STREAMS", "colors", ">")
			c.Do("XAUTOCLAIM", "colors", "pr", "alice", "0", "0", "JUSTID")
			c.Do("XREADGROUP", "GROUP", "pr", "alice", "STREAMS", "colors", ">")
			c.Do("XAUTOCLAIM", "colors", "pr", "alice", "0", "0", "JUSTID")
			c.Do("XAUTOCLAIM", "colors", "pr", "alice", "0", "0", "COUNT", "1", "JUSTID")
			c.Do("XPENDING", "colors", "pr")

			c.Do("XAUTOCLAIM", "colors", "pr", "eve", "0", "0", "JUSTID")
			c.Do("XPENDING", "colors", "pr")

			c.Error("syntax error", "XAUTOCLAIM", "colors", "pr", "alice", "0", "0", "JUSTID", "foo")
			c.Error("No such key", "XAUTOCLAIM", "colors", "foo", "alice", "0", "0", "JUSTID")
			c.Error("No such key", "XAUTOCLAIM", "foo", "pr", "alice", "0", "0", "JUSTID")
			c.Error("Invalid min-idle-time", "XAUTOCLAIM", "colors", "pr", "alice", "foo", "0", "JUSTID")
			c.Error("Invalid stream ID", "XAUTOCLAIM", "colors", "pr", "alice", "0", "foo", "JUSTID")
			c.Error("Invalid stream ID", "XAUTOCLAIM", "colors", "pr", "alice", "0", "-1", "JUSTID")
		})

		// regular mode
		testRaw(t, func(c *client) {
			c.Do("XGROUP", "CREATE", "colors", "pr", "$", "MKSTREAM")
			c.Do("XAUTOCLAIM", "colors", "pr", "alice", "0", "0")
			c.Do("XADD", "colors", "42-2", "name", "Green")
			c.Do("XADD", "colors", "42-3", "name", "Blue")
			c.Do("XREADGROUP", "GROUP", "pr", "alice", "STREAMS", "colors", ">")
			c.Do("XAUTOCLAIM", "colors", "pr", "alice", "0", "0")
			c.Do("XREADGROUP", "GROUP", "pr", "alice", "STREAMS", "colors", ">")
			c.Do("XAUTOCLAIM", "colors", "pr", "alice", "0", "0")
			c.Do("XAUTOCLAIM", "colors", "pr", "alice", "0", "0", "COUNT", "1")

			c.Do("XAUTOCLAIM", "colors", "pr", "eve", "0", "0")
			c.Do("XPENDING", "colors", "pr")
		})
	})

	t.Run("XCLAIM", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Error("No such key", "XCLAIM", "planets", "processing", "alice", "0", "0-0")
			c.Do("XGROUP", "CREATE", "planets", "processing", "$", "MKSTREAM")
			c.Error("No such key", "XCLAIM", "planets", "foo", "alice", "0", "0-0")
			c.Do("XCLAIM", "planets", "processing", "alice", "0", "0-0")
			c.DoLoosely("XINFO", "CONSUMERS", "planets", "processing") // "idle" is fiddly

			c.Do("XADD", "planets", "0-1", "name", "Mercury")
			c.Do("XADD", "planets", "0-2", "name", "Venus")

			c.Do("XCLAIM", "planets", "processing", "alice", "0", "0-1")
			c.DoLoosely("XINFO", "CONSUMERS", "planets", "processing") //  "idle" is fiddly

			c.Do("XCLAIM", "planets", "processing", "alice", "0", "0-1", "0-2", "FORCE")
			c.Do("XINFO", "GROUPS", "planets")
			c.Do("XPENDING", "planets", "processing")

			c.Do("XDEL", "planets", "0-1") // !
			c.Do("XCLAIM", "planets", "processing", "bob", "0", "0-1")
			c.Do("XINFO", "GROUPS", "planets")
			c.Do("XPENDING", "planets", "processing")

			c.Do("XADD", "planets", "0-3", "name", "Mercury")
			c.Do("XADD", "planets", "0-4", "name", "Venus")

			c.Do("XCLAIM", "planets", "processing", "bob", "0", "0-4", "FORCE")
			c.Do("XCLAIM", "planets", "processing", "bob", "0", "0-4")
			c.Do("XPENDING", "planets", "processing")

			c.Do("XREADGROUP", "GROUP", "processing", "alice", "COUNT", "1", "STREAMS", "planets", ">")
			c.Do("XPENDING", "planets", "processing")
			c.Do("XREADGROUP", "GROUP", "processing", "alice", "STREAMS", "planets", ">")
			c.Do("XPENDING", "planets", "processing")

			c.Do("XCLAIM", "planets", "processing", "alice", "0", "0-3", "RETRYCOUNT", "10", "IDLE", "5000", "JUSTID")
			c.Do("XCLAIM", "planets", "processing", "alice", "0", "0-1", "0-2", "RETRYCOUNT", "1", "TIME", "1", "JUSTID")
			c.Do("XCLAIM", "planets", "processing", "alice", "0", "0-1", "0-4", "RETRYCOUNT", "1", "TIME", "1", "justid")
			c.Do("XPENDING", "planets", "processing")

			c.Do("XACK", "planets", "processing", "0-1", "0-2", "0-3", "0-4")
			c.Do("XPENDING", "planets", "processing")

			c.Error("Unrecognized XCLAIM option", "XCLAIM", "planets", "processing", "alice", "0", "0-3", "RETRYCOUNT", "10", "0-4", "IDLE", "0")
			c.Error("Unrecognized XCLAIM option", "XCLAIM", "planets", "processing", "alice", "0", "0-3", "RETRYCOUNT", "10", "IDLE", "0", "0-4")
			c.Error("Invalid min-idle-time", "XCLAIM", "planets", "processing", "alice", "foo", "0-1", "JUSTID")
			c.Error("Invalid IDLE", "XCLAIM", "planets", "processing", "alice", "0", "0-1", "JUSTID", "IDLE", "foo")
			c.Error("Invalid TIME", "XCLAIM", "planets", "processing", "alice", "0", "0-1", "JUSTID", "TIME", "foo")
			c.Error("Invalid RETRYCOUNT", "XCLAIM", "planets", "processing", "alice", "0", "0-1", "JUSTID", "RETRYCOUNT", "foo")
		})
	})

	testRESP3(t, func(c *client) {
		c.DoLoosely("XINFO", "STREAM", "foo")
	})
}

func TestStreamTrim(t *testing.T) {
	skip(t)
	t.Run("XTRIM MAXLEN", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("XADD", "planets", "0-1", "name", "Mercury")
			c.Do("XADD", "planets", "1-0", "name", "Venus")
			c.Do("XADD", "planets", "2-1", "name", "Earth")
			c.Do("XADD", "planets", "3-0", "name", "Mars")
			c.Do("XADD", "planets", "4-1", "name", "Jupiter")

			c.Do("XTRIM", "planets", "MAXLEN", "3")
			c.Do("XRANGE", "planets", "-", "+")

			c.Do("XTRIM", "planets", "MAXLEN", "=", "3")
			c.Do("XRANGE", "planets", "-", "+")

			c.Do("XTRIM", "planets", "MAXLEN", "2")
			c.Do("XRANGE", "planets", "-", "+")

			c.Do("XTRIM", "planets", "MAXLEN", "~", "2", "LIMIT", "99")
			c.Do("XRANGE", "planets", "-", "+")

			// error cases
			c.Error("not an integer", "XTRIM", "planets", "MAXLEN", "abc")
			c.Error("arguments", "XTRIM", "planets", "MAXLEN")
			c.Error("without the special", "XTRIM", "planets", "MAXLEN", "3", "LIMIT", "1")
		})
	})

	t.Run("XTRIM MINID", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("XADD", "planets", "0-1", "name", "Mercury")
			c.Do("XADD", "planets", "1-0", "name", "Venus")
			c.Do("XADD", "planets", "2-1", "name", "Earth")
			c.Do("XADD", "planets", "3-0", "name", "Mars")
			c.Do("XADD", "planets", "4-1", "name", "Jupiter")

			c.Do("XTRIM", "planets", "MINID", "1")
			c.Do("XRANGE", "planets", "-", "+")

			c.Do("XTRIM", "planets", "MINID", "=", "1")
			c.Do("XRANGE", "planets", "-", "+")

			c.Do("XTRIM", "planets", "MINID", "3")
			c.Do("XRANGE", "planets", "-", "+")

			c.Do("XTRIM", "planets", "MINID", "~", "3", "LIMIT", "1")
			c.Do("XRANGE", "planets", "-", "+")

			// error cases
			c.Error("arguments", "XTRIM", "planets", "MINID")
			c.Error("arguments", "XTRIM", "planets")
			c.Error("arguments", "XTRIM", "planets", "OTHER")
			c.Error("without the special", "XTRIM", "planets", "MINID", "3", "LIMIT", "1")
			c.Error("out of range", "XTRIM", "planets", "MINID", "~", "3", "LIMIT", "one")
		})
	})
}
