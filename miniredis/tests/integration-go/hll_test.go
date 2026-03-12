package main

import (
	"math/rand"
	"testing"
)

func TestHll(t *testing.T) {
	skip(t)
	t.Run("basics", func(t *testing.T) {
		testRaw(t, func(c *client) {
			// Add 100 unique random values to h1 and 50 of these 100 to h2
			for i := 0; i < 100; i++ {
				value := randomStr(10)
				c.Do("PFADD", "h1", value)
				if i%2 == 0 {
					c.Do("PFADD", "h2", value)
				}
			}

			for i := 0; i < 100; i++ {
				c.Do("PFADD", "h3", randomStr(10))
			}

			// Merge non-intersecting hlls
			{
				c.Do(
					"PFMERGE",
					"res1",
					"h1", // count 100
					"h3", // count 100
				)
				c.DoApprox(2, "PFCOUNT", "res1")
			}

			// Merge intersecting hlls
			{
				c.Do(
					"PFMERGE",
					"res2",
					"h1", // count 100
					"h2", // count 50 (all 50 are presented in h1)
				)
				c.DoApprox(2, "PFCOUNT", "res2")
			}

			// Merge all hlls
			{
				c.Do(
					"PFMERGE",
					"res3",
					"h1", // count 100
					"h2", // count 50 (all 50 are presented in h1)
					"h3", // count 100
					"h4", // empty key
				)
				c.DoApprox(2, "PFCOUNT", "res3")
			}

			// failure cases
			c.Error("wrong number", "PFADD")
			c.Error("wrong number", "PFCOUNT")
			c.Error("wrong number", "PFMERGE")
			c.Do("SET", "str", "I am a string")
			c.Error("not a valid HyperLogLog", "PFADD", "str", "noot", "mies")
			c.Error("not a valid HyperLogLog", "PFCOUNT", "str", "h1")
			c.Error("not a valid HyperLogLog", "PFMERGE", "str", "noot")
			c.Error("not a valid HyperLogLog", "PFMERGE", "noot", "str")

			c.Do("DEL", "h1", "h2", "h3", "h4", "res1", "res2", "res3")
			c.Do("PFCOUNT", "h1", "h2", "h3", "h4", "res1", "res2", "res3")
		})
	})

	t.Run("tx", func(t *testing.T) {
		testRaw(t, func(c *client) {
			c.Do("MULTI")
			c.Do("PFADD", "h1", "noot", "mies", "vuur", "wim")
			c.Do("PFADD", "h2", "noot1", "mies1", "vuur1", "wim1")
			c.Do("PFMERGE", "h3", "h1", "h2")
			c.Do("PFCOUNT", "h1")
			c.Do("PFCOUNT", "h2")
			c.Do("PFCOUNT", "h3")
			c.Do("EXEC")
		})
	})
}

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randomStr(length int) string {
	rand.Seed(42)
	b := make([]byte, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
