package main

import (
	"testing"
)

func TestGeoadd(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("GEOADD",
			"Sicily",
			"13.361389", "38.115556", "Palermo",
			"15.087269", "37.502669", "Catania",
		)
		c.Do("ZRANGE", "Sicily", "0", "-1")
		c.Do("ZRANGE", "Sicily", "0", "-1", "WITHSCORES")

		c.Do("GEOADD",
			"mountains",
			"86.9248308", "27.9878675", "Everest",
			"142.1993050", "11.3299030", "Challenger Deep",
			"31.132", "29.976", "Pyramids",
		)
		c.Do("ZRANGE", "mountains", "0", "-1")
		c.Do("GEOADD", // re-add an existing one
			"mountains",
			"86.9248308", "27.9878675", "Everest",
		)
		c.Do("ZRANGE", "mountains", "0", "-1")
		c.Do("GEOADD", // update
			"mountains",
			"86.9248308", "28.000", "Everest",
		)
		c.Do("ZRANGE", "mountains", "0", "-1")

		// failure cases
		c.Error("invalid", "GEOADD", "err", "186.9248308", "27.9878675", "not the Everest")
		c.Error("invalid", "GEOADD", "err", "-186.9248308", "27.9878675", "not the Everest")
		c.Error("invalid", "GEOADD", "err", "86.9248308", "87.9878675", "not the Everest")
		c.Error("invalid", "GEOADD", "err", "86.9248308", "-87.9", "not the Everest")
		c.Do("SET", "str", "I am a string")
		c.Error("wrong kind", "GEOADD", "str", "86.9248308", "27.9878675", "Everest")
		c.Error("wrong number", "GEOADD")
		c.Error("wrong number", "GEOADD", "foo")
		c.Error("wrong number", "GEOADD", "foo", "86.9248308")
		c.Error("wrong number", "GEOADD", "foo", "86.9248308", "27.9878675")
		c.Do("GEOADD", "foo", "86.9248308", "27.9878675", "")
		c.Error("not a valid float", "GEOADD", "foo", "eight", "27.9878675", "bar")
		c.Error("not a valid float", "GEOADD", "foo", "86.9248308", "seven", "bar")
		// failures in a transaction
		c.Do("MULTI")
		c.Error("wrong number", "GEOADD", "foo")
		c.Error("discarded", "EXEC")
		c.Do("MULTI")
		c.Do("GEOADD", "foo", "eight", "27.9878675", "bar")
		c.Do("EXEC")

		// 2nd key is invalid
		c.Do("MULTI")
		c.Do("GEOADD", "two",
			"86.9248308", "28.000", "Everest",
			"eight", "27.9878675", "bar",
		)
		c.Do("EXEC")
		c.Do("ZRANGE", "two", "0", "-1")
	})
}

func TestGeopos(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("GEOADD",
			"Sicily",
			"13.361389", "38.115556", "Palermo",
			"15.087269", "37.502669", "Catania",
		)
		c.Do("GEOPOS", "Sicily")
		c.DoRounded(3, "GEOPOS", "Sicily", "Palermo")
		c.Do("GEOPOS", "Sicily", "nosuch")
		c.DoRounded(3, "GEOPOS", "Sicily", "Catania", "Palermo")
		c.DoRounded(3, "GEOPOS", "Sicily", "Catania", "Catania", "Palermo")
		c.Do("GEOPOS", "nosuch", "Palermo")

		// failure cases
		c.Error("wrong number", "GEOPOS")
		c.Do("SET", "foo", "bar")
		c.Error("wrong kind", "GEOPOS", "foo", "Palermo")
	})
}

func TestGeodist(t *testing.T) {
	skip(t)
	testRaw(t, func(c *client) {
		c.Do("GEOADD",
			"Sicily",
			"13.361389", "38.115556", "Palermo",
			"15.087269", "37.502669", "Catania",
		)
		c.DoRounded(2, "GEODIST", "Sicily", "Palermo", "Catania")
		c.DoRounded(2, "GEODIST", "Sicily", "Catania", "Palermo")
		c.Do("GEODIST", "Sicily", "nosuch", "Palermo")
		c.Do("GEODIST", "Sicily", "Catania", "nosuch")
		c.Do("GEODIST", "nosuch", "Catania", "Palermo")
		c.DoRounded(2, "GEODIST", "Sicily", "Palermo", "Catania", "m")
		c.Do("GEODIST", "Sicily", "Palermo", "Catania", "km")
		c.Do("GEODIST", "Sicily", "Palermo", "Catania", "KM")
		c.Do("GEODIST", "Sicily", "Palermo", "Catania", "mi")
		c.DoRounded(2, "GEODIST", "Sicily", "Palermo", "Catania", "ft")
		c.Do("GEODIST", "Sicily", "Palermo", "Palermo")

		c.Error("unsupported unit", "GEODIST", "Sicily", "Palermo", "Palermo", "yards")
		c.Error("wrong number", "GEODIST")
		c.Error("wrong number", "GEODIST", "Sicily")
		c.Error("wrong number", "GEODIST", "Sicily", "Palermo")
		c.Error("syntax error", "GEODIST", "Sicily", "Palermo", "Palermo", "miles", "too many")
		c.Error("unsupported unit provided. please use M, KM, FT, MI", "GEODIST", "Sicily", "Palermo", "Catania", "foobar")
		c.Do("SET", "string", "123")
		c.Error("wrong kind", "GEODIST", "string", "a", "b")
	})
}

func TestGeoradius(t *testing.T) {
	skip(t)
	t.Run("basic", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("GEOADD",
				"stations",
				"-73.99106999861966", "40.73005400028978", "Astor Pl",
				"-74.00019299927328", "40.71880300107709", "Canal St",
				"-73.98384899986625", "40.76172799961419", "50th St",
			)
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "4", "km")
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "4", "KM")
			c.Do("GEORADIUS", "stations", "1.0", "1.0", "1", "km")
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "4", "ft", "WITHDIST")
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "4", "m", "WITHDIST")
			// redis has more precision in the coords
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "4", "m", "WITHCOORD")
			c.DoRounded(3, "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "WITHDIST", "WITHCOORD")
			c.DoRounded(3, "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "WITHCOORD", "WITHDIST")
			c.DoRounded(3, "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "WITHCOORD", "WITHCOORD", "WITHCOORD")
			c.DoRounded(3, "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "WITHDIST", "WITHDIST", "WITHDIST")
			// FIXME: the distances don't quite match for miles or km
			c.DoRounded(3, "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "mi", "WITHDIST")
			c.DoRounded(3, "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "WITHDIST")

			// Sorting
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "DESC")
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "ASC")
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "ASC", "DESC", "ASC")

			// COUNT
			c.DoRounded(3, "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "ASC", "COUNT", "1")
			c.DoRounded(3, "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "ASC", "COUNT", "2")
			c.DoRounded(3, "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "ASC", "COUNT", "999")
			c.Error("syntax error", "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "COUNT")
			c.Error("COUNT must", "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "COUNT", "0")
			c.Error("COUNT must", "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "COUNT", "-12")
			c.Error("not an integer", "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "COUNT", "foobar")

			// non-existing key
			c.Do("GEORADIUS", "foo", "-73.9718893", "40.7728773", "4", "km")

			// no error in redis, for some reason
			// c.Do("GEORADIUS", "foo", "-73.9718893", "40.7728773", "4", "km", "FOOBAR")
			c.Error("syntax error", "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "ASC", "FOOBAR")

			// GEORADIUS_RO
			c.Do("GEORADIUS_RO", "stations", "-73.9718893", "40.7728773", "4", "km")
			c.Do("GEORADIUS_RO", "stations", "1.0", "1.0", "1", "km")
			c.Error("syntax error", "GEORADIUS_RO", "stations", "-73.9718893", "40.7728773", "4", "km", "STORE", "bar")
			c.Error("syntax error", "GEORADIUS_RO", "stations", "-73.9718893", "40.7728773", "4", "km", "STOREDIST", "bar")
			c.Error("syntax error", "GEORADIUS_RO", "stations", "-73.9718893", "40.7728773", "4", "km", "STORE")
			c.Error("syntax error", "GEORADIUS_RO", "stations", "-73.9718893", "40.7728773", "4", "km", "STOREDIST")
		})
	})

	t.Run("STORE", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("GEOADD",
				"stations",
				"-73.99106999861966", "40.73005400028978", "Astor Pl",
				"-74.00019299927328", "40.71880300107709", "Canal St",
				"-73.98384899986625", "40.76172799961419", "50th St",
			)

			// plain store
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "STORE", "foo")
			c.Do("ZRANGE", "foo", "0", "-1")
			c.Do("ZRANGE", "foo", "0", "-1", "WITHSCORES")

			// Yeah, valid:
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "STORE", "")
			c.Do("ZRANGE", "", "0", "-1")

			// store with count, and overwrite existing key
			c.Do("EXPIRE", "foo", "999")
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "ASC", "COUNT", "1", "STORE", "foo")
			c.Do("ZRANGE", "foo", "0", "-1")
			c.Do("TTL", "foo")

			// store should overwrite
			c.Do("SET", "taken", "123")
			c.Do("EXPIRE", "taken", "999")
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "STORE", "taken")
			c.Do("TYPE", "taken")
			c.Do("ZRANGE", "taken", "0", "-1")
			c.Do("TTL", "taken")

			// errors
			c.Error("syntax error", "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "STORE")
			c.Error("not compatible", "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "WITHDIST", "STORE", "foo")
			c.Error("not compatible", "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "WITHCOORD", "STORE", "foo")
		})
	})

	t.Run("STOREDIST", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("GEOADD",
				"stations",
				"-73.99106999861966", "40.73005400028978", "Astor Pl",
				"-74.00019299927328", "40.71880300107709", "Canal St",
				"-73.98384899986625", "40.76172799961419", "50th St",
			)

			// plain store
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "STOREDIST", "foo")
			c.Do("ZRANGE", "foo", "0", "-1")
			c.DoRounded(3, "ZRANGE", "foo", "0", "-1", "WITHSCORES")

			// plain store, meter
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "m", "STOREDIST", "meter")
			c.Do("ZRANGE", "meter", "0", "-1")
			c.DoRounded(3, "ZRANGE", "meter", "0", "-1", "WITHSCORES")

			// Yeah, valid:
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "STOREDIST", "")
			c.Do("ZRANGE", "", "0", "-1")

			// STOREDIST with count
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "ASC", "COUNT", "1", "STOREDIST", "foo")
			c.Do("ZRANGE", "foo", "0", "-1")

			// store should overwrite
			c.Do("SET", "taken", "123")
			c.Do("EXPIRE", "taken", "9999")
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "STOREDIST", "taken")
			c.Do("TYPE", "taken")
			c.Do("ZRANGE", "taken", "0", "-1")
			c.Do("TTL", "taken")

			// multiple keys
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "STOREDIST", "n1", "STOREDIST", "n2", "STOREDIST", "n3")
			c.Do("TYPE", "n1")
			c.Do("TYPE", "n2")
			c.Do("TYPE", "n3")

			// STORE and STOREDIST
			c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "STOREDIST", "a", "STORE", "b")
			c.Do("TYPE", "a")
			c.Do("ZRANGE", "a", "0", "-1")
			c.Do("TYPE", "b")
			c.Do("ZRANGE", "b", "0", "-1")

			// errors
			c.Error("syntax error", "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "STOREDIST")
			c.Error("not compatible", "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "WITHDIST", "STOREDIST", "foo")
			c.Error("not compatible", "GEORADIUS", "stations", "-73.9718893", "40.7728773", "400", "km", "WITHCOORD", "STOREDIST", "foo")
		})
	})
}

func TestGeoradiusByMember(t *testing.T) {
	skip(t)
	t.Run("basic", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("GEOADD",
				"stations",
				"-73.99106999861966", "40.73005400028978", "Astor Pl",
				"-74.00019299927328", "40.71880300107709", "Canal St",
				"-73.98384899986625", "40.76172799961419", "50th St",
			)
			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "4", "km")
			// c.Do("GEORADIUSBYMEMBER", "stations", "1.0", "1.0", "1", "km") // Not valid test

			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "4", "ft", "WITHDIST")
			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "4", "m", "WITHDIST")
			// redis has more precision in the coords
			c.DoRounded(3, "GEORADIUSBYMEMBER", "stations", "Astor Pl", "4", "m", "WITHCOORD")
			c.DoRounded(3, "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "WITHDIST", "WITHCOORD")
			c.DoRounded(3, "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "WITHCOORD", "WITHDIST")
			c.DoRounded(3, "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "WITHCOORD", "WITHCOORD", "WITHCOORD")
			c.DoRounded(3, "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "WITHDIST", "WITHDIST", "WITHDIST")
			// FIXME: the distances don't quite match for miles or km
			c.DoRounded(3, "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "mi", "WITHDIST")
			c.DoRounded(3, "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "WITHDIST")

			// Sorting
			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "DESC")
			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "ASC")
			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "ASC", "DESC", "ASC")

			// COUNT
			c.DoRounded(3, "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "ASC", "COUNT", "1")
			c.DoRounded(3, "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "ASC", "COUNT", "2")
			c.DoRounded(3, "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "ASC", "COUNT", "999")
			c.Error("syntax error", "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "COUNT")
			c.Error("COUNT must", "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "COUNT", "0")
			c.Error("COUNT must", "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "COUNT", "-12")
			c.Error("not an integer", "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "COUNT", "foobar")

			// non-existing key
			// c.Do("GEORADIUSBYMEMBER", "foo", "Astor Pl", "4", "km") // Failing
			// geo_test.go:268: value error. expected: []interface {}{} got: <nil> case: main.command{cmd:"GEORADIUSBYMEMBER", args:[]interface {}{"foo", "Astor Pl", "4", "km"}, error:false, sort:false, loosely:false, errorSub:"", receiveOnly:false, roundFloats:0, closeChan:false}

			// no error in redis, for some reason
			// c.Do("GEORADIUSBYMEMBER", "foo", "Astor Pl", "4", "km", "FOOBAR")
			c.Error("syntax error", "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "ASC", "FOOBAR")

			// GEORADIUSBYMEMBER_RO
			c.Do("GEORADIUSBYMEMBER_RO", "stations", "Astor Pl", "4", "km")
			// c.Do("GEORADIUSBYMEMBER_RO", "stations", "1.0", "1.0", "1", "km") // Not a valid test
			c.Error("syntax error", "GEORADIUSBYMEMBER_RO", "stations", "Astor Pl", "4", "km", "STORE", "bar")
			c.Error("syntax error", "GEORADIUSBYMEMBER_RO", "stations", "Astor Pl", "4", "km", "STOREDIST", "bar")
			c.Error("syntax error", "GEORADIUSBYMEMBER_RO", "stations", "Astor Pl", "4", "km", "STORE")
			c.Error("syntax error", "GEORADIUSBYMEMBER_RO", "stations", "Astor Pl", "4", "km", "STOREDIST")
		})
	})

	t.Run("STORE", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("GEOADD",
				"stations",
				"-73.99106999861966", "40.73005400028978", "Astor Pl",
				"-74.00019299927328", "40.71880300107709", "Canal St",
				"-73.98384899986625", "40.76172799961419", "50th St",
			)

			// plain store
			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "STORE", "foo")
			c.Do("ZRANGE", "foo", "0", "-1")
			c.Do("ZRANGE", "foo", "0", "-1", "WITHSCORES")

			// Yeah, valid:
			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "STORE", "")
			c.Do("ZRANGE", "", "0", "-1")

			// store with count, and overwrite existing key
			c.Do("EXPIRE", "foo", "999")
			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "ASC", "COUNT", "1", "STORE", "foo")
			c.Do("ZRANGE", "foo", "0", "-1")
			c.Do("TTL", "foo")

			// store should overwrite
			c.Do("SET", "taken", "123")
			c.Do("EXPIRE", "taken", "999")
			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "STORE", "taken")
			c.Do("TYPE", "taken")
			c.Do("ZRANGE", "taken", "0", "-1")
			c.Do("TTL", "taken")

			// errors
			c.Error("syntax error", "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "STORE")
			c.Error("not compatible", "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "WITHDIST", "STORE", "foo")
			c.Error("not compatible", "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "WITHCOORD", "STORE", "foo")
		})
	})

	t.Run("STOREDIST", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("GEOADD",
				"stations",
				"-73.99106999861966", "40.73005400028978", "Astor Pl",
				"-74.00019299927328", "40.71880300107709", "Canal St",
				"-73.98384899986625", "40.76172799961419", "50th St",
			)

			// plain store
			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "STOREDIST", "foo")
			c.Do("ZRANGE", "foo", "0", "-1")
			c.DoRounded(3, "ZRANGE", "foo", "0", "-1", "WITHSCORES")

			// plain store, meter
			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "m", "STOREDIST", "meter")
			c.Do("ZRANGE", "meter", "0", "-1")
			c.DoRounded(3, "ZRANGE", "meter", "0", "-1", "WITHSCORES")

			// Yeah, valid:
			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "STOREDIST", "")
			c.Do("ZRANGE", "", "0", "-1")

			// STOREDIST with count
			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "ASC", "COUNT", "1", "STOREDIST", "foo")
			c.Do("ZRANGE", "foo", "0", "-1")

			// store should overwrite
			c.Do("SET", "taken", "123")
			c.Do("EXPIRE", "taken", "9999")
			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "STOREDIST", "taken")
			c.Do("TYPE", "taken")
			c.Do("ZRANGE", "taken", "0", "-1")
			c.Do("TTL", "taken")

			// multiple keys
			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "STOREDIST", "n1", "STOREDIST", "n2", "STOREDIST", "n3")
			c.Do("TYPE", "n1")
			c.Do("TYPE", "n2")
			c.Do("TYPE", "n3")

			// STORE and STOREDIST
			c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "STOREDIST", "a", "STORE", "b")
			c.Do("TYPE", "a")
			c.Do("ZRANGE", "a", "0", "-1")
			c.Do("TYPE", "b")
			c.Do("ZRANGE", "b", "0", "-1")

			// errors
			c.Error("syntax error", "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "STOREDIST")
			c.Error("not compatible", "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "WITHDIST", "STOREDIST", "foo")
			c.Error("not compatible", "GEORADIUSBYMEMBER", "stations", "Astor Pl", "400", "km", "WITHCOORD", "STOREDIST", "foo")
		})
	})
}

// a bit longer testset
func TestGeo(t *testing.T) {
	skip(t)
	// some subway stations
	// https://data.cityofnewyork.us/Transportation/Subway-Stations/arq3-7z49/data
	testRaw(t, func(c *client) {
		c.Do("GEOADD",
			"stations",
			"-73.99106999861966", "40.73005400028978", "Astor Pl",
			"-74.00019299927328", "40.71880300107709", "Canal St",
			"-73.98384899986625", "40.76172799961419", "50th St",
			"-73.97499915116808", "40.68086213682956", "Bergen St",
			"-73.89488591154061", "40.66471445143568", "Pennsylvania Ave",
			"-73.90087000018522", "40.88466700064975", "238th St",
			"-73.95806670661364", "40.800581558114956", "Cathedral Pkwy (110th St)",
			"-73.94085899871263", "40.67991899941601", "Kingston - Throop Aves",
			"-73.8987883783301", "40.74971952935675", "65th St",
			"-73.92901818461539", "40.75196004401078", "36th St",
			"-73.98740940202974", "40.71830605618619", "Delancey St - Essex St",
			"-73.89165772702445", "40.67802821447783", "Van Siclen Ave",
			"-73.87962599910783", "40.68152000045683", "Norwood Ave",
			"-73.84443500029684", "40.69516599823373", "104th-102nd Sts",
			"-73.98177094440949", "40.690648119969794", "DeKalb Ave",
			"-73.82758075034528", "40.58326843810286", "Beach 105th St",
			"-73.81365140419632", "40.58809156457325", "Beach 90th St",
			"-73.89175225349464", "40.829987446384116", "Freeman St",
			"-73.89661738461646", "40.822142131170786", "Intervale Ave",
			"-73.90074099998965", "40.85609299881864", "182nd-183rd Sts",
			"-73.91013600050078", "40.84589999983414", "174th-175th Sts",
			"-73.91843200082253", "40.83376899862797", "167th St",
			"-73.8456249984179", "40.75462199881262", "Mets - Willets Point",
			"-73.86952700103515", "40.74914499948836", "Junction Blvd",
			"-73.83003000262508", "40.75959999915012", "Flushing - Main St",
			"-73.83256900003744", "40.846809998885504", "Buhre Ave",
			"-73.92613800014134", "40.81047600117261", "3rd Ave - 138th St",
			"-73.85122199961472", "40.83425499825462", "Castle Hill Ave",
			"-74.0041310005885", "40.713064999433136", "Brooklyn Bridge - City Hall",
			"-73.8470359987544", "40.836488000608156", "Zerega Ave",
			"-73.9767132992584", "40.75180742981634", "Grand Central - 42nd St",
			"-73.98207600148947", "40.74608099909145", "33rd St",
			"-73.9510700015425", "40.78567199998607", "96th St",
			"-73.95987399886047", "40.77362000074615", "77th St",
			"-73.91038357033376", "40.68285130087804", "Chauncey St",
			"-73.98310999909673", "40.67731566735096", "Union St",
			"-73.8820347465864", "40.74237007972169", "Elmhurst Ave",
			"-73.92078599933306", "40.678822000873375", "Ralph Ave",
			"-73.86748067850041", "40.8571924091606", "Pelham Pkwy",
			"-73.86613410538703", "40.877839385172024", "Gun Hill Rd",
			"-73.8543153107622", "40.898286515575286", "Nereid Ave (238 St)",
			"-73.9580997367769", "40.67076515344894", "Franklin Ave",
			"-73.89306639507903", "40.823976841237396", "Simpson St",
			"-73.86835609178098", "40.848768666338934", "Bronx Park East",
			"-73.95007934590994", "40.65665931376077", "Winthrop St",
			"-73.88940491730106", "40.665517963059635", "Van Siclen Ave",
			"-73.9273847542618", "40.81830344372315", "149th St - Grand Concourse",
			"-73.92569199505733", "40.82823032742169", "161st St - Yankee Stadium",
			"-73.9679670004732", "40.762526000304575", "Lexington Ave - 59th St",
			"-73.90409799875945", "40.81211799827203", "E 149th St",
			"-73.87451599929486", "40.82952100156747", "Morrison Av - Soundview",
			"-73.8862829985325", "40.82652500055904", "Whitlock Ave",
			"-73.86761799923673", "40.8315090005233", "St Lawrence Ave",
			"-73.90298400173006", "40.745630001138395", "Woodside - 61st St",
			"-73.75540499924732", "40.603995001687544", "Far Rockaway - Mott Ave",
			"-73.976336575218", "40.77551939729258", "72nd St",
			"-73.96460245687166", "40.79161879767014", "96th St",
			"-73.93956099985425", "40.84071899990795", "168th St",
			"-73.8935090000331", "40.86697799999945", "Kingsbridge Rd",
			"-73.98459099904711", "40.754184001312545", "42nd St - Bryant Pk",
			"-73.96203130426609", "40.6616334551018", "Prospect Park",
			"-73.99534882595742", "40.63147876093745", "55th St",
			"-73.81701287135405", "40.70289855287313", "Jamaica - Van Wyck",
			"-73.8303702709878", "40.714034819571026", "Kew Gardens - Union Tpke",
			"-73.80800471963833", "40.700382424235", "Sutphin Blvd - Archer Av",
			"-73.94605470266329", "40.747768121414325", "Court Sq - 23rd St",
			"-73.85286048434907", "40.726505475813006", "67th Ave",
			"-73.87722085669182", "40.736813418197144", "Grand Ave - Newtown",
			"-73.97817199965161", "40.63611866666291", "Ditmas Ave",
			"-73.95999000137212", "40.68888900026455", "Classon Ave",
			"-73.95031225606621", "40.706126576274166", "Broadway",
			"-73.95024799996972", "40.71407200064717", "Lorimer St",
			"-73.9019160004208", "40.66914500061398", "Sutter Ave",
			"-73.90395860491864", "40.68886654246024", "Wilson Ave",
			"-73.9166388842194", "40.686415270704344", "Halsey St",
			"-73.94735499884204", "40.703844000042096", "Lorimer St",
			"-74.01151599772157", "40.634970999647166", "8th Ave",
			"-73.929861999118", "40.7564420005104", "36th Ave",
			"-73.92582299919906", "40.761431998800546", "Broadway",
			"-73.98676800153976", "40.75461199851542", "Times Sq - 42nd St",
			"-73.97918899989101", "40.75276866674217", "Grand Central - 42nd St",
			"-73.95762400074634", "40.67477166685263", "Park Pl",
			"-73.83216299845388", "40.68433100001238", "111th St",
			"-74.00030814755975", "40.732254493367876", "W 4th St - Washington Sq (Lower)",
			"-73.97192000069982", "40.75710699989316", "51st St",
			"-73.97621799859327", "40.78864400073892", "86th St",
			"-73.85736239521543", "40.89314324138378", "233rd St",
			"-73.98220899995783", "40.77344000052039", "66th St - Lincoln Ctr",
			"-73.89054900017344", "40.82094799852307", "Hunts Point Ave",
			"-74.0062770001748", "40.72285399778783", "Canal St",
			"-73.83632199755944", "40.84386300128381", "Middletown Rd",
			"-73.98659900207888", "40.739864000474604", "23rd St",
			"-73.94526400039679", "40.74702299889643", "Court Sq",
			"-73.98192900232715", "40.76824700063689", "59th St - Columbus Circle",
			"-73.9489160009391", "40.74221599986316", "Hunters Point Ave",
			"-73.9956570016487", "40.74408099989751", "23rd St",
			"-74.00536700180581", "40.728251000730204", "Houston St",
			"-73.83768300060997", "40.681711001091195", "104th St",
			"-73.81583268782963", "40.60840218069683", "Broad Channel",
			"-73.96850099975177", "40.57631166708091", "Ocean Pkwy",
			"-73.95358099875249", "40.74262599969749", "Vernon Blvd - Jackson Ave",
			"-73.96387000158042", "40.76814100049679", "68th St - Hunter College",
			"-73.9401635351909", "40.750635651014804", "Queensboro Plz",
			"-73.8438529979573", "40.680428999588415", "Rockaway Blvd",
			"-73.98995099881881", "40.734673000996125", "Union Sq - 14th St",
			"-73.95352200064022", "40.68962700158444", "Bedford - Nostrand Aves",
			"-73.97973580592873", "40.66003568810021", "15th St - Prospect Park",
			"-73.98025117900944", "40.66624469001985", "7th Ave",
			"-73.97577599917474", "40.65078166803418", "Ft Hamilton Pkwy",
			"-73.97972116229084", "40.64427200012998", "Church Ave",
			"-73.96435779623125", "40.64390459860419", "Beverly Rd",
			"-73.96288246192114", "40.65049324646484", "Church Ave",
			"-73.96269486837261", "40.63514193733789", "Newkirk Ave",
			"-73.96145343987648", "40.65507304163716", "Parkside Ave",
			"-73.9709563319228", "40.6752946951032", "Grand Army Plaza",
			"-73.97754993539385", "40.68442016526762", "Atlantic Av - Barclay's Center",
			"-73.91194599726617", "40.678339999883505", "Rockaway Ave",
			"-73.97537499833149", "40.68711899950771", "Fulton St",
			"-73.9667959986695", "40.68809400106055", "Clinton - Washington Aves",
			"-73.97285279191024", "40.67710217983294", "7th Ave",
			"-73.97678343963167", "40.684488323453685", "Atlantic Av - Barclay's Center",
			"-73.97880999956767", "40.683665667279435", "Atlantic Av - Barclay's Center",
			"-73.99015100090539", "40.692403999991036", "Borough Hall",
			"-73.83591899965162", "40.672096999172844", "Aqueduct Racetrack",
			"-73.86049500117254", "40.85436399966426", "Morris Park",
			"-73.85535900043564", "40.858984999820116", "Pelham Pkwy",
			"-73.95042600099683", "40.68043800006226", "Nostrand Ave",
			"-73.98040679874578", "40.68831058019022", "Nevins St",
			"-73.96422203748425", "40.67203223545925", "Eastern Pkwy - Bklyn Museum",
			"-73.94884798381702", "40.64512351894373", "Beverly Rd",
			"-73.94945514035334", "40.6508606878022", "Church Ave",
			"-73.94829990822407", "40.63999124275311", "Newkirk Ave",
			"-73.94754120734406", "40.63284240700742", "Brooklyn College - Flatbush Ave",
			"-73.95072891124937", "40.6627729934283", "Sterling St",
			"-73.93293256081851", "40.66897831107809", "Crown Hts - Utica Ave",
			"-73.94215978392963", "40.66948144864978", "Kingston Ave",
			"-73.95118300016523", "40.724479997808274", "Nassau Ave",
			"-73.95442500146235", "40.73126699971465", "Greenpoint Ave",
			"-73.95783200075729", "40.708383000017925", "Marcy Ave",
			"-73.95348800038457", "40.706889998054", "Hewes St",
			"-73.92984899935611", "40.81322399958908", "138th St - Grand Concourse",
			"-73.9752485052734", "40.76008683231326", "5th Ave - 53rd St",
			"-73.96907237490204", "40.75746830782865", "Lexington Ave - 53rd St",
			"-73.98869800128737", "40.74545399979951", "28th St",
			"-73.9879368338264", "40.74964456009442", "Herald Sq - 34th St",
			"-73.98168087489128", "40.73097497580066", "1st Ave",
			"-73.98622899953202", "40.755983000570076", "Times Sq - 42nd St",
			"-73.9514239994525", "40.71277400073426", "Metropolitan Ave",
			"-73.94049699874644", "40.71157600064823", "Grand St",
			"-73.94394399869037", "40.714575998363635", "Graham Ave",
			"-73.95666499806525", "40.71717399858899", "Bedford Ave",
			"-73.93979284713505", "40.70739106438455", "Montrose Ave",
			"-73.94381559597835", "40.74630503357145", "Long Island City - Court Sq",
			"-73.9495999997552", "40.7441286664954", "21st St",
			"-73.93285137679598", "40.75276306140845", "39th Ave",
			"-73.95035999879713", "40.82655099962194", "145th St",
			"-73.94488999901047", "40.8340410001399", "157th St",
			"-73.97232299915696", "40.79391900121471", "96th St",
			"-73.96837899960818", "40.799446000334825", "103rd St",
			"-73.95182200176913", "40.79907499977324", "Central Park North (110th St)",
			"-73.96137008267617", "40.796060739904526", "103rd St",
			"-73.98197000159583", "40.77845300068614", "72nd St",
			"-73.97209794937208", "40.78134608418206", "81st St",
			"-73.83692369387158", "40.71804465348743", "75th Ave",
			"-73.96882849429672", "40.78582304678557", "86th St",
			"-73.9668470005456", "40.80396699961484", "Cathedral Pkwy (110th St)",
			"-73.96410999757751", "40.807722001230864", "116th St - Columbia University",
			"-73.94549500011411", "40.807753999182815", "125th St",
			"-73.94077000106708", "40.8142290003391", "135th St",
			"-73.94962500096905", "40.802097999133004", "116th St",
			"-73.90522700122354", "40.850409999510234", "Tremont Ave",
			"-73.95367600087873", "40.82200799968475", "137th St - City College",
			"-73.93624499873299", "40.82042099969279", "145th St",
			"-73.91179399884471", "40.8484800012369", "176th St",
			"-73.9076840015997", "40.85345300155693", "Burnside Ave",
			"-73.91339999846983", "40.83930599964156", "170th St",
			"-73.94013299907257", "40.840555999148535", "168th St",
			"-73.9335959996056", "40.84950499974065", "181st St",
			"-73.92941199742039", "40.85522500175836", "191st St",
			"-73.93970399761596", "40.84739100072403", "175th St",
			"-73.77601299999507", "40.59294299908617", "Beach 44th St",
			"-73.7885219980118", "40.59237400121235", "Beach 60th St",
			"-73.82052058959523", "40.58538569133279", "Beach 98th St",
			"-73.83559008701239", "40.580955865573515", "Rockaway Park - Beach 116 St",
			"-73.76817499939688", "40.59539800166876", "Beach 36th St",
			"-73.76135299762073", "40.60006600105881", "Beach 25th St",
			"-73.80328900021885", "40.707571999615695", "Parsons Blvd",
			"-73.79347419927721", "40.710517502784", "169th St",
			"-73.86269999830412", "40.749865000555545", "103rd St - Corona Plaza",
			"-73.85533399834884", "40.75172999941711", "111th St",
			"-73.86161820097203", "40.729763972422425", "63rd Dr - Rego Park",
			"-73.86504999877702", "40.67704400054478", "Grant Ave",
			"-73.97991700056134", "40.78393399959032", "79th St",
			"-73.9030969995401", "40.67534466640805", "Atlantic Ave",
			"-74.00290599855235", "40.73342200104225", "Christopher St - Sheridan Sq",
			"-73.82579799906613", "40.68595099878361", "Ozone Park - Lefferts Blvd",
			"-73.98769099825152", "40.755477001982506", "Times Sq - 42nd St",
			"-73.97595787413822", "40.576033818103646", "W 8th St - NY Aquarium",
			"-73.99336500134324", "40.74721499918219", "28th St",
			"-73.98426400110407", "40.743069999259035", "28th St",
			"-73.82812100059289", "40.85246199951662", "Pelham Bay Park",
			"-73.84295199925012", "40.839892001013915", "Westchester Sq - E Tremont Ave",
			"-73.99787100060406", "40.741039999802105", "18th St",
			"-73.97604100111508", "40.751431000286864", "Grand Central - 42nd St",
			"-73.7969239998421", "40.59092700078133", "Beach 67th St",
			"-74.00049500225435", "40.73233799774325", "W 4th St - Washington Sq (Upper)",
			"-73.86008700006875", "40.69242699966103", "85th St - Forest Pky",
			"-73.85205199740794", "40.69370399880105", "Woodhaven Blvd",
			"-73.83679338454697", "40.697114810696476", "111th St",
			"-73.82834900017954", "40.700481998515315", "121st St",
			"-73.90393400118631", "40.69551800114878", "Halsey St",
			"-73.9109757182647", "40.699471062427136", "Myrtle - Wyckoff Aves",
			"-73.88411070800329", "40.6663149325969", "New Lots Ave",
			"-73.8903580002471", "40.67270999906104", "Van Siclen Ave",
			"-73.8851940021643", "40.679777998961164", "Cleveland St",
			"-73.90056237226057", "40.66405727094644", "Livonia Ave",
			"-73.90244864183562", "40.66358900181724", "Junius St",
			"-73.90895833584449", "40.66261748815223", "Rockaway Ave",
			"-73.90185000017287", "40.64665366739528", "Canarsie - Rockaway Pkwy",
			"-73.89954769388724", "40.65046878544699", "E 105th St",
			"-73.91633025007947", "40.6615297898075", "Saratoga Ave",
			"-73.92252118536001", "40.66476678877493", "Sutter Ave - Rutland Road",
			"-73.89927796057142", "40.65891477368527", "New Lots Ave",
			"-73.90428999746412", "40.67936600147369", "Broadway Junction",
			"-73.89852600159652", "40.676998000003756", "Alabama Ave",
			"-73.88074999747269", "40.6741300014559", "Shepherd Ave",
			"-73.87392925215778", "40.68315265707736", "Crescent St",
			"-73.87332199882995", "40.689616000838754", "Cypress Hills",
			"-73.86728799944963", "40.691290001246735", "75th St - Eldert Ln",
			"-73.8964029993185", "40.746324999410284", "69th St",
			"-73.8912051289911", "40.746867573829114", "74th St - Broadway",
			"-73.86943208612348", "40.73309737380972", "Woodhaven Blvd - Queens Mall",
			"-73.91217899939602", "40.69945400090837", "Myrtle - Wyckoff Aves",
			"-73.90758199885423", "40.70291899894902", "Seneca Ave",
			"-73.91823200219723", "40.70369299961644", "DeKalb Ave",
			"-73.91254899891254", "40.744149001021576", "52nd St",
			"-73.91352174995538", "40.756316952608096", "46th St",
			"-73.90606508052358", "40.752824829236076", "Northern Blvd",
			"-73.91843500103973", "40.74313200060382", "46th St",
			"-73.88369700071884", "40.747658999559135", "82nd St - Jackson Hts",
			"-73.87661299986985", "40.74840800060913", "90th St - Elmhurst Av",
			"-73.83030100071032", "40.66047600004959", "Howard Beach - JFK Airport",
			"-73.83405799948723", "40.668234001699815", "Aqueduct - North Conduit Av",
			"-73.82069263637443", "40.70916181536946", "Briarwood - Van Wyck Blvd",
			"-73.84451672012669", "40.72159430953587", "Forest Hills - 71st Av",
			"-73.81083299897232", "40.70541799906764", "Sutphin Blvd",
			"-73.80109632298924", "40.70206737621188", "Jamaica Ctr - Parsons / Archer",
			"-73.86021461772737", "40.88802825863786", "225th St",
			"-73.87915899874777", "40.82858400108929", "Elder Ave",
			"-73.89643499897414", "40.816103999972405", "Longwood Ave",
			"-73.91809500109238", "40.77003699949086", "Astoria Blvd",
			"-73.9120340001031", "40.775035666523664", "Astoria - Ditmars Blvd",
			"-73.9077019387083", "40.81643746686396", "Jackson Ave",
			"-73.90177778730917", "40.81948726483844", "Prospect Ave",
			"-73.91404199994753", "40.8053680007636", "Cypress Ave",
			"-73.88769359812888", "40.837195550170605", "174th St",
			"-73.86723422851625", "40.86548337793927", "Allerton Ave",
			"-73.90765699936489", "40.80871900090143", "E 143rd St - St Mary's St",
			"-73.89717400101743", "40.867760000885795", "Kingsbridge Rd",
			"-73.89006400069478", "40.87341199980121", "Bedford Park Blvd - Lehman College",
			"-73.93647000005559", "40.82388000080457", "Harlem - 148 St",
			"-73.9146849986034", "40.84443400092679", "Mt Eden Ave",
			"-73.89774900102401", "40.861295998683495", "Fordham Rd",
			"-73.91779099745928", "40.84007499993004", "170th St",
			"-73.88713799889574", "40.87324399861646", "Bedford Park Blvd",
			"-73.90983099923551", "40.87456099941789", "Marble Hill - 225th St",
			"-73.90483400107873", "40.87885599817935", "231st St",
			"-73.91527899954356", "40.86944399946045", "215th St",
			"-73.91881900132312", "40.864614000525854", "207th St",
			"-73.91989900100465", "40.86807199999737", "Inwood - 207th St",
			"-73.89858300049647", "40.88924800011476", "Van Cortlandt Park - 242nd St",
			"-73.87996127877184", "40.84020763241799", "West Farms Sq - E Tremont Av",
			"-73.8625097078866", "40.883887974625274", "219th St",
			"-73.88465499988732", "40.87974999947229", "Mosholu Pkwy",
			"-73.87885499918691", "40.87481100011182", "Norwood - 205th St",
			"-73.86705361747603", "40.87125880254771", "Burke Ave",
			"-73.83859099802153", "40.87866300037311", "Baychester Ave",
			"-73.8308340021742", "40.88829999901007", "Eastchester - Dyre Ave",
			"-73.78381700176453", "40.712645666744045", "Jamaica - 179th St",
			"-73.8506199987954", "40.903125000541245", "Wakefield - 241st St",
			"-73.95924499945693", "40.670342666584396", "Botanic Garden",
			"-73.90526176305106", "40.68286062551184", "Bushwick - Aberdeen",
			"-73.90311757920684", "40.67845624842869", "Broadway Junction",
			"-73.84638400151765", "40.86952599962676", "Gun Hill Rd",
			"-73.87334609510884", "40.8418630412186", "E 180th St",
			"-73.92553600006474", "40.86053100138796", "Dyckman St",
			"-73.95837200097044", "40.815580999978934", "125th St",
			"-73.95582700110425", "40.68059566598263", "Franklin Ave - Fulton St",
			"-73.92672247438611", "40.81833014409742", "149th St - Grand Concourse",
			"-73.91779152760981", "40.816029252510006", "3rd Ave - 149th St",
			"-73.92139999784426", "40.83553699933672", "167th St",
			"-73.91923999909432", "40.80756599987699", "Brook Ave",
			"-73.93099699953838", "40.74458699983993", "33rd St",
			"-73.9240159984882", "40.74378100149132", "40th St",
			"-73.94408792823116", "40.824766360871905", "145th St",
			"-73.93820899811622", "40.8301349999812", "155th St",
			"-73.92565099775477", "40.827904998845845", "161st St - Yankee Stadium",
			"-73.93072899914027", "40.67936399950546", "Utica Ave",
			"-73.9205264716827", "40.75698735912575", "Steinway St",
			"-73.92850899927413", "40.69317200129202", "Kosciuszko St",
			"-73.92215600150752", "40.689583999013905", "Gates Ave",
			"-73.92724299902838", "40.69787300011831", "Central Ave",
			"-73.91972000188625", "40.69866000123805", "Knickerbocker Ave",
			"-73.9214790001739", "40.76677866673298", "30th Ave",
			"-73.9229130000312", "40.706606665988716", "Jefferson St",
			"-73.93314700024209", "40.70615166680729", "Morgan Ave",
			"-73.93713823965695", "40.74891771986323", "Queens Plz",
			"-73.97697099965796", "40.62975466638584", "18th Ave",
			"-74.0255099996266", "40.629741666886915", "77th St",
			"-74.02337699950728", "40.63496666682377", "Bay Ridge Ave",
			"-73.9946587805514", "40.636260890961395", "50th St",
			"-74.00535100046275", "40.63138566722445", "Ft Hamilton Pkwy",
			"-73.98682900011477", "40.59770366695856", "25th Ave",
			"-73.9936762000529", "40.601950461572315", "Bay Pky",
			"-73.98452199846113", "40.617108999866005", "20th Ave",
			"-73.99045399865993", "40.620686997680025", "18th Ave",
			"-74.03087600085765", "40.61662166725951", "Bay Ridge - 95th St",
			"-74.0283979999864", "40.62268666715025", "86th St",
			"-74.00058287431507", "40.61315892569516", "79th St",
			"-73.99884094850685", "40.61925870977273", "71st St",
			"-73.99817432157568", "40.60467699816932", "20th Ave",
			"-74.00159259239406", "40.60773573171741", "18th Ave",
			"-73.99685724994863", "40.626224462922195", "62nd St",
			"-73.99635300025969", "40.62484166725887", "New Utrecht Ave",
			"-73.97337641974885", "40.59592482551748", "Ave U",
			"-73.9723553085244", "40.603258405128265", "Kings Hwy",
			"-73.96135378598797", "40.577710196642435", "Brighton Beach",
			"-73.95405791257907", "40.58654754707536", "Sheepshead Bay",
			"-73.95581122316301", "40.59930895095475", "Ave U",
			"-73.95760873538083", "40.608638645396006", "Kings Hwy",
			"-73.97908400099428", "40.597235999920436", "Ave U",
			"-73.98037300229343", "40.60405899980493", "Kings Hwy",
			"-73.97459272818807", "40.580738758491464", "Neptune Ave",
			"-73.97426599968905", "40.589449666625285", "Ave X",
			"-73.98376500045946", "40.58884066651933", "Bay 50th St",
			"-73.97818899936274", "40.59246500088859", "Gravesend - 86th St",
			"-73.97300281528751", "40.608842808949916", "Ave P",
			"-73.97404850873143", "40.61435671190883", "Ave N",
			"-73.9752569782215", "40.62073162316788", "Bay Pky",
			"-73.9592431052215", "40.617397744443736", "Ave M",
			"-73.98178001069293", "40.61145578989005", "Bay Pky",
			"-73.97606933170925", "40.62501744019143", "Ave I",
			"-73.96069316246925", "40.625022819915166", "Ave J",
			"-73.96151793942495", "40.62920837758969", "Ave H",
			"-73.95507827493762", "40.59532169111695", "Neck Rd",
			"-73.94193761457447", "40.75373927087553", "21st St - Queensbridge",
			"-73.98598400026407", "40.76245599925997", "50th St",
			"-73.98169782344476", "40.76297015245628", "7th Ave",
			"-73.98133100227702", "40.75864100159815", "47th-50th Sts - Rockefeller Ctr",
			"-73.97736800085171", "40.76408500081713", "57th St",
			"-73.96608964413245", "40.76461809442373", "Lexington Ave - 63rd St",
			"-73.95323499978866", "40.75917199967108", "Roosevelt Island - Main St",
			"-73.98164872301398", "40.768249531776064", "59th St - Columbus Circle",
			"-73.98420956591096", "40.759801973870694", "49th St",
			"-73.98072973372128", "40.76456552501829", "57th St",
			"-73.97334700047045", "40.764810999755284", "5th Ave - 59th St",
			"-73.96737501711436", "40.762708855394564", "Lexington Ave - 59th St",
			"-73.99105699913983", "40.75037300003949", "34th St - Penn Station",
			"-73.98749500051885", "40.75528999995681", "Times Sq - 42nd St",
			"-74.00762309323994", "40.71016216530185", "Fulton St",
			"-74.00858473570133", "40.714111000774025", "Chambers St",
			"-73.98973500085859", "40.757307998551504", "42nd St - Port Authority Bus Term",
			"-73.94906699890156", "40.69461899903765", "Myrtle-Willoughby Aves",
			"-73.9502340010257", "40.70037666622154", "Flushing Ave",
			"-73.99276500471389", "40.742954317826005", "23rd St",
			"-73.98777189072918", "40.74978939990011", "Herald Sq - 34th St",
			"-73.98503624034139", "40.68840847580642", "Hoyt - Schermerhorn Sts",
			"-73.98721815267317", "40.692470636847084", "Jay St - MetroTech",
			"-73.99017700122197", "40.713855001020406", "East Broadway",
			"-73.98807806807719", "40.71868074219453", "Delancey St - Essex St",
			"-73.98993800003434", "40.72340166574911", "Lower East Side - 2nd Ave",
			"-73.94137734838365", "40.70040440298112", "Flushing Ave",
			"-73.9356230012996", "40.6971950005145", "Myrtle Ave",
			"-73.98977899938897", "40.67027166728493", "4th Av - 9th St",
			"-73.99589172790934", "40.67364106090412", "Smith - 9th Sts",
			"-73.99075649573565", "40.68611054725977", "Bergen St",
			"-73.98605667854612", "40.69225539645323", "Jay St - MetroTech",
			"-73.99181830901125", "40.694196480776995", "Court St",
			"-73.99053886181645", "40.73587226699812", "Union Sq - 14th St",
			"-73.98934400102907", "40.74130266729", "23rd St",
			"-73.99287200067424", "40.66541366712979", "Prospect Ave",
			"-73.98830199974512", "40.670846666842756", "4th Av - 9th St",
			"-73.98575000112093", "40.73269099971662", "3rd Ave",
			"-73.99066976901818", "40.73476331217923", "Union Sq - 14th St",
			"-73.89654800103929", "40.67454199987086", "Liberty Ave",
			"-73.90531600055341", "40.67833366608023", "Broadway Junction",
			"-74.01788099953987", "40.6413616662838", "59th St",
			"-74.01000600074939", "40.648938666612814", "45th St",
			"-74.00354899951809", "40.65514366633887", "36th St",
			"-73.99444874451204", "40.64648407726636", "9th Ave",
			"-74.01403399986317", "40.64506866735981", "53rd St",
			"-73.9942022375285", "40.640912711444656", "Ft Hamilton Pkwy",
			"-73.99809099974297", "40.66039666692321", "25th St",
			"-73.99494697998841", "40.68027335170176", "Carroll St",
			"-74.00373899843763", "40.72622700129312", "Spring St",
			"-73.93796900205011", "40.851694999744616", "181st St",
			"-73.93417999964333", "40.85902199892482", "190th St",
			"-73.95479778057312", "40.80505813344211", "116th St",
			"-73.95224799734774", "40.811071672994565", "125th St",
			"-73.99770200045987", "40.72432866597571", "Prince St",
			"-73.99250799849149", "40.73046499853991", "8th St - NYU",
			"-74.00657099970202", "40.70941599925865", "Fulton St",
			"-74.00881099997359", "40.713050999077694", "Park Pl",
			"-74.00926600170112", "40.71547800011327", "Chambers St",
			"-73.98506379575646", "40.69054418535472", "Hoyt St",
			"-73.98999799960687", "40.693218999611084", "Borough Hall",
			"-73.90387900151532", "40.85840700040842", "183rd St",
			"-73.90103399921699", "40.86280299988937", "Fordham Rd",
			"-74.00974461517701", "40.71256392680817", "World Trade Center",
			"-74.0052290023424", "40.72082400007119", "Canal St - Holland Tunnel",
			"-73.94151400082208", "40.83051799929251", "155th St",
			"-73.93989200188344", "40.83601299923096", "163rd St - Amsterdam Av",
			"-74.00793800110387", "40.71002266658424", "Fulton St",
			"-74.00340673031336", "40.71323378962671", "Chambers St",
			"-73.99982638545937", "40.71817387697391", "Canal St",
			"-74.00698581780337", "40.71327233111697", "City Hall",
			"-74.0018260000577", "40.71946500105898", "Canal St",
			"-74.01316895919258", "40.701730507574474", "South Ferry",
			"-74.01400799803432", "40.70491399928076", "Bowling Green",
			"-74.01186199860112", "40.70755700086603", "Wall St",
			"-74.0130072374272", "40.703142373599135", "Whitehall St",
			"-74.01297456253795", "40.707744756294474", "Rector St",
			"-73.8958980017196", "40.70622599823048", "Fresh Pond Rd",
			"-73.88957722978091", "40.711431305058255", "Middle Village - Metropolitan Ave",
			"-74.01378300119742", "40.707512999521775", "Rector St",
			"-74.01218800112292", "40.7118350008202", "Cortlandt St",
			"-74.00950899856461", "40.710367998822136", "Fulton St",
			"-74.01105599991755", "40.706476001106005", "Broad St",
			"-74.01113196473266", "40.7105129841524", "Cortlandt St",
			"-74.00909999844257", "40.706820999753376", "Wall St",
			"-73.92727099960726", "40.865490998968916", "Dyckman St",
			"-73.99375299913589", "40.71826699954992", "Grand St",
			"-73.99620399876055", "40.725296998738045", "Broadway - Lafayette St",
			"-73.99380690654237", "40.720246883147254", "Bowery",
			"-74.00105471306033", "40.718814263587134", "Canal St",
			"-73.99804100117201", "40.74590599939995", "23rd St",
			"-73.99339099970578", "40.752287000775894", "34th St - Penn Station",
			"-73.89129866519697", "40.74653969115889", "Jackson Hts - Roosevelt Av",
			"-74.00020100063497", "40.737825999728116", "14th St",
			"-73.94753480879213", "40.817905559212676", "135th St",
			"-73.99620899921355", "40.73822799969515", "14th St",
			"-73.99775078874781", "40.73774146981052", "6th Ave",
			"-74.00257800104762", "40.73977666638199", "8th Ave",
			"-74.00168999937027", "40.740893000193296", "14th St",
			"-73.9504262489579", "40.66993815093054", "Nostrand Ave",
			"-73.99308599821961", "40.69746599996469", "Clark St",
			"-73.95684800014614", "40.68137966658742", "Franklin Ave",
			"-73.96583799857275", "40.68326299912644", "Clinton - Washington Aves",
			"-73.90307500005954", "40.70441200087814", "Forest Ave",
			"-73.94424999687163", "40.795020000113105", "110th St",
			"-73.95558899985132", "40.77949199820952", "86th St",
			"-73.98688499993673", "40.699742667691574", "York St",
			"-73.99053100065458", "40.69933699977884", "High St",
			"-73.97394599849406", "40.68611300020567", "Lafayette Ave",
			"-73.95058920022207", "40.667883603536815", "President St",
			"-73.87875099990931", "40.886037000253324", "Woodlawn",
			"-73.99465900006331", "40.72591466682659", "Bleecker St",
			"-73.94747800152219", "40.79060000008452", "103rd St",
			"-73.87210600099675", "40.675376998239365", "Euclid Ave",
			"-73.85147000026086", "40.67984300135503", "88th St",
			"-73.96379005505493", "40.6409401651401", "Cortelyou Rd",
			"-73.9416169983714", "40.7986290002001", "116th St",
			"-73.86081600108396", "40.83322599927859", "Parkchester",
			"-74.00688600277107", "40.719318001302135", "Franklin St",
			"-73.85899200206335", "40.67937100115432", "80th St",
			"-73.98196299856706", "40.75382100064824", "5th Ave - Bryant Pk",
			"-73.99714100006673", "40.72230099999366", "Spring St",
			"-73.93759400055725", "40.804138000587244", "125th St",
			"-73.9812359981396", "40.57728100006751", "Coney Island - Stillwell Av",
			"-74.00219709442206", "40.75544635961596", "34th St - Hudson Yards",
			"-73.95836178682246", "40.76880251014895", "72nd St",
			"-73.95177090964917", "40.77786104333163", "86th St",
			"-73.9470660219183", "40.784236650177654", "96th St",
		)
		c.Do("ZRANGE", "stations", "0", "-1")
		c.Do("ZRANGE", "stations", "0", "-1", "WITHSCORES")
		c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "4", "km")
		c.DoRounded(3, "GEORADIUS", "stations", "-73.9718893", "40.7728773", "4", "km", "WITHDIST")
		c.DoRounded(3, "GEORADIUS", "stations", "-73.9718893", "40.7728773", "4", "km", "WITHCOORD")
		c.DoRounded(3, "GEORADIUS", "stations", "-73.9718893", "40.7728773", "4", "km", "ASC")
		c.DoRounded(3, "GEORADIUS", "stations", "-73.9718893", "40.7728773", "4", "km", "DESC")
		c.DoRounded(3, "GEORADIUS", "stations", "-73.9718893", "40.7728773", "4", "km", "DESC", "COUNT", "3")
		c.DoRounded(3, "GEORADIUS", "stations", "-73.9718893", "40.7728773", "4", "km", "ASC", "COUNT", "3")
		c.DoRounded(3, "GEORADIUS", "stations", "-73.9718893", "40.7728773", "4", "km", "ASC", "COUNT", "99999")

		c.DoSorted("GEORADIUSBYMEMBER", "stations", "Astor Pl", "4", "km")
		c.DoLoosely("GEORADIUSBYMEMBER", "stations", "Astor Pl", "4", "km", "WITHDIST")
		c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "4", "km", "WITHDIST", "ASC")
		c.DoLoosely("GEORADIUSBYMEMBER", "stations", "Astor Pl", "4", "km", "WITHCOORD")
		c.DoRounded(3, "GEORADIUSBYMEMBER", "stations", "Astor Pl", "4", "km", "WITHCOORD", "ASC")
		c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "4", "km", "ASC")
		c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "4", "km", "DESC")
		c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "4", "km", "DESC", "COUNT", "3")
		c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "4", "km", "ASC", "COUNT", "3")
		c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "4", "km", "ASC", "COUNT", "99999")

		c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "4", "km", "STORE", "res")
		c.Do("ZRANGE", "res", "0", "-1", "WITHSCORES")
		c.Do("GEORADIUS", "stations", "-73.9718893", "40.7728773", "4", "km", "STOREDIST", "resd")
		c.DoLoosely("ZRANGE", "resd", "0", "-1", "WITHSCORES")

		c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "4", "km", "STORE", "resbymem")
		c.Do("ZRANGE", "resbymem", "0", "-1", "WITHSCORES")
		c.Do("GEORADIUSBYMEMBER", "stations", "Astor Pl", "4", "km", "STOREDIST", "resbymemd")
		c.DoLoosely("ZRANGE", "resbymemd", "0", "-1", "WITHSCORES")
	})
}
