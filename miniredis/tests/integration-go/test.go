package main

import (
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/alicebob/miniredis/v2/proto"
)

func skip(t testing.TB) {
	t.Helper()
	if os.Getenv("INT") == "" {
		t.Skip("INT=1 not set")
	}
}

func testRaw(t *testing.T, cb func(*client)) {
	t.Helper()

	sMini := miniredis.RunT(t)

	sReal, sRealAddr := Redis()
	t.Cleanup(sReal.Close)

	client := newClient(t, sRealAddr, sMini)

	cb(client)
}

// like testRaw, but with two connections
func testRaw2(t *testing.T, cb func(*client, *client)) {
	t.Helper()

	sMini := miniredis.RunT(t)

	sReal, sRealAddr := Redis()
	t.Cleanup(sReal.Close)

	client1 := newClient(t, sRealAddr, sMini)
	client2 := newClient(t, sRealAddr, sMini)

	cb(client1, client2)
}

// like testRaw2, but with connections in Go routines
func testMulti(t *testing.T, cbs ...func(*client)) {
	t.Helper()

	sMini := miniredis.RunT(t)

	sReal, sRealAddr := Redis()
	t.Cleanup(sReal.Close)

	var wg sync.WaitGroup
	for _, cb := range cbs {
		wg.Add(1)
		go func(cb func(*client)) {
			client := newClient(t, sRealAddr, sMini)
			cb(client)
			wg.Done()
		}(cb)
	}
	wg.Wait()
}

// similar to testRaw, but redis runs with authentication enabled
func testAuth(t *testing.T, passwd string, cb func(*client)) {
	t.Helper()

	sMini := miniredis.RunT(t)
	sMini.RequireAuth(passwd)

	sReal, sRealAddr := RedisAuth(passwd)
	t.Cleanup(sReal.Close)

	client := newClient(t, sRealAddr, sMini)

	cb(client)
}

// similar to testAuth, but redis runs with redis6 multiuser authentication enabled
func testUserAuth(t *testing.T, users map[string]string, cb func(*client)) {
	t.Helper()

	sMini := miniredis.RunT(t)
	for user, pass := range users {
		sMini.RequireUserAuth(user, pass)
	}

	sReal, sRealAddr := RedisUserAuth(users)
	t.Cleanup(sReal.Close)

	client := newClient(t, sRealAddr, sMini)

	cb(client)
}

// similar to testRaw, but redis is started in cluster mode
func testCluster(t *testing.T, cb func(*client)) {
	t.Helper()

	sMini := miniredis.RunT(t)

	sReal, sRealAddr := RedisCluster()
	t.Cleanup(sReal.Close)

	client := newClient(t, sRealAddr, sMini)

	cb(client)
}

// similar to testRaw, but connections require TLS
func testTLS(t *testing.T, cb func(*client)) {
	t.Helper()

	sMini := miniredis.NewMiniRedis()
	if err := sMini.StartTLS(testServerTLS(t)); err != nil {
		t.Fatalf("unexpected miniredis error: %s", err.Error())
	}
	t.Cleanup(sMini.Close)

	sReal, sRealAddr := RedisTLS()
	t.Cleanup(sReal.Close)

	client := newClientTLS(t, sRealAddr, sMini)

	cb(client)
}

// like testRaw, but switched to RESP3 protocol.
func testRESP3(t *testing.T, cb func(*client)) {
	t.Helper()

	sMini := miniredis.RunT(t)

	sReal, sRealAddr := Redis()
	t.Cleanup(sReal.Close)

	client := newClientResp3(t, sRealAddr, sMini)

	cb(client)
}

// like testRESP3, but with two connections
func testRESP3Pair(t *testing.T, cb func(*client, *client)) {
	t.Helper()

	sMini := miniredis.RunT(t)

	sReal, sRealAddr := Redis()
	t.Cleanup(sReal.Close)

	client1 := newClientResp3(t, sRealAddr, sMini)
	client2 := newClientResp3(t, sRealAddr, sMini)

	cb(client1, client2)
}

func looselyEqual(a, b interface{}) bool {
	switch av := a.(type) {
	case string:
		_, ok := b.(string)
		return ok
	case []byte:
		_, ok := b.([]byte)
		return ok
	case int64:
		_, ok := b.(int64)
		return ok
	case int:
		_, ok := b.(int)
		return ok
	case error:
		_, ok := b.(error)
		return ok
	case []interface{}:
		bv, ok := b.([]interface{})
		if !ok {
			return false
		}
		if len(av) != len(bv) {
			return false
		}
		for i, v := range av {
			if !looselyEqual(v, bv[i]) {
				return false
			}
		}
		return true
	case map[interface{}]interface{}:
		bv, ok := b.(map[interface{}]interface{})
		if !ok {
			return false
		}
		if len(av) != len(bv) {
			return false
		}
		for k, v := range av {
			if !looselyEqual(v, bv[k]) {
				return false
			}
		}
		return true
	default:
		panic(fmt.Sprintf("unhandled case, got a %#v / %T", a, a))
	}
}

// round all floats
func roundFloats(r interface{}, pos int) interface{} {
	switch ls := r.(type) {
	case []interface{}:
		var new []interface{}
		for _, k := range ls {
			new = append(new, roundFloats(k, pos))
		}
		return new
	case []byte:
		f, err := strconv.ParseFloat(string(ls), 64)
		if err != nil {
			return ls
		}
		return []byte(fmt.Sprintf("%.[1]*f", pos, f))
	case string:
		f, err := strconv.ParseFloat(string(ls), 64)
		if err != nil {
			return ls
		}
		return fmt.Sprintf("%.[1]*f", pos, f)
	default:
		fmt.Printf("unhandled type: %T FIXME\n", r)
		return nil
	}
}

// client which compares two redises
type client struct {
	t          *testing.T
	real, mini *proto.Client
	miniredis  *miniredis.Miniredis // in case you need m.FastForward() and friends
}

func newClient(t *testing.T, realAddr string, mini *miniredis.Miniredis) *client {
	t.Helper()

	cReal, err := proto.Dial(realAddr)
	if err != nil {
		t.Fatalf("realredis: %s", err.Error())
	}

	cMini, err := proto.Dial(mini.Addr())
	if err != nil {
		t.Fatalf("miniredis: %s", err.Error())
	}

	return &client{
		t:         t,
		miniredis: mini,
		real:      cReal,
		mini:      cMini,
	}
}

func newClientTLS(t *testing.T, realAddr string, mini *miniredis.Miniredis) *client {
	t.Helper()

	cfg := testClientTLS(t)

	cReal, err := proto.DialTLS(
		realAddr,
		cfg,
	)
	if err != nil {
		t.Fatalf("realredis: %s", err.Error())
	}

	cMini, err := proto.DialTLS(
		mini.Addr(),
		cfg,
	)
	if err != nil {
		t.Fatalf("miniredis: %s", err.Error())
	}

	return &client{
		t:         t,
		miniredis: mini,
		real:      cReal,
		mini:      cMini,
	}
}

func newClientResp3(t *testing.T, realAddr string, mini *miniredis.Miniredis) *client {
	t.Helper()

	cReal, err := proto.Dial(realAddr)
	if err != nil {
		t.Fatalf("realredis: %s", err.Error())
	}
	if _, err := cReal.Do("HELLO", "3"); err != nil {
		t.Fatalf("realredis HELLO: %s", err.Error())
	}

	cMini, err := proto.Dial(mini.Addr())
	if err != nil {
		t.Fatalf("miniredis: %s", err.Error())
	}
	if _, err := cMini.Do("HELLO", "3"); err != nil {
		t.Fatalf("miniredis HELLO: %s", err.Error())
	}

	return &client{
		t:         t,
		miniredis: mini,
		real:      cReal,
		mini:      cMini,
	}
}

// Do() is the main test function. The given redis command is executed on both
// a real redis and on miniredis, and the returned results must be exactly the
// same. See the other Do... commands for variants which are more flexible in
// their comparison.
func (c *client) Do(cmd string, args ...string) {
	c.t.Helper()

	resReal, errReal := c.real.Do(append([]string{cmd}, args...)...)
	if errReal != nil {
		c.t.Errorf("error from realredis: %s", errReal)
		return
	}
	resMini, errMini := c.mini.Do(append([]string{cmd}, args...)...)
	if errMini != nil {
		c.t.Errorf("error from miniredis: %s", errMini)
		return
	}

	// c.t.Logf("real:%q mini:%q", string(resReal), string(resMini))

	if resReal != resMini {
		c.t.Errorf("real: %q mini: %q", string(resReal), string(resMini))
		return
	}

	if strings.HasPrefix(string(resReal), "-") {
		c.t.Errorf("Do() returned a redis error, use c.Error(): %q", string(resReal))
	}
}

// result must be []string, and we'll sort them before comparing
func (c *client) DoSorted(cmd string, args ...string) {
	c.t.Helper()

	resReal, errReal := c.real.Do(append([]string{cmd}, args...)...)
	if errReal != nil {
		c.t.Errorf("error from realredis: %s", errReal)
		return
	}
	resMini, errMini := c.mini.Do(append([]string{cmd}, args...)...)
	if errMini != nil {
		c.t.Errorf("error from miniredis: %s", errMini)
		return
	}

	// c.t.Logf("real:%q mini:%q", string(resReal), string(resMini))
	realStrings, err := proto.ReadStrings(resReal)
	if err != nil {
		c.t.Errorf("readstrings realredis: %s", errReal)
		return
	}
	miniStrings, err := proto.ReadStrings(resMini)
	if err != nil {
		c.t.Errorf("readstrings miniredis: %s", errReal)
		return
	}

	sort.Strings(realStrings)
	sort.Strings(miniStrings)

	if !reflect.DeepEqual(realStrings, miniStrings) {
		c.t.Errorf("expected: %q got: %q", realStrings, miniStrings)
	}
}

// result must kinda match (just the structure, exact values are not compared)
func (c *client) DoLoosely(cmd string, args ...string) {
	c.t.Helper()

	resReal, errReal := c.real.Do(append([]string{cmd}, args...)...)
	if errReal != nil {
		c.t.Errorf("error from realredis: %s", errReal)
		return
	}
	resMini, errMini := c.mini.Do(append([]string{cmd}, args...)...)
	if errMini != nil {
		c.t.Errorf("error from miniredis: %s", errMini)
		return
	}

	// c.t.Logf("real:%q mini:%q", string(resReal), string(resMini))

	mini, err := proto.Parse(resMini)
	if err != nil {
		c.t.Errorf("parse error miniredis: %s", err)
		return
	}
	real, err := proto.Parse(resReal)
	if err != nil {
		c.t.Errorf("parse error realredis: %s", err)
		return
	}
	if !looselyEqual(real, mini) {
		c.t.Errorf("expected a loose match want: %#v have: %#v", real, mini)
	}
}

// result must match, with floats rounded
func (c *client) DoRounded(rounded int, cmd string, args ...string) {
	c.t.Helper()

	resReal, errReal := c.real.Do(append([]string{cmd}, args...)...)
	if errReal != nil {
		c.t.Errorf("error from realredis: %s", errReal)
		return
	}
	resMini, errMini := c.mini.Do(append([]string{cmd}, args...)...)
	if errMini != nil {
		c.t.Errorf("error from miniredis: %s", errMini)
		return
	}

	// c.t.Logf("real:%q mini:%q", string(resReal), string(resMini))

	mini, err := proto.Parse(resMini)
	if err != nil {
		c.t.Errorf("parse error miniredis: %s", err)
		return
	}
	real, err := proto.Parse(resReal)
	if err != nil {
		c.t.Errorf("parse error realredis: %s", err)
		return
	}
	real = roundFloats(real, rounded)
	mini = roundFloats(mini, rounded)
	if !reflect.DeepEqual(real, mini) {
		c.t.Errorf("expected a match (rounded to %d) want: %#v have: %#v", rounded, real, mini)
	}
}

// result must be a single int, with value within threshold
func (c *client) DoApprox(threshold int, cmd string, args ...string) {
	c.t.Helper()

	resReal, errReal := c.real.Do(append([]string{cmd}, args...)...)
	if errReal != nil {
		c.t.Errorf("error from realredis: %s", errReal)
		return
	}
	resMini, errMini := c.mini.Do(append([]string{cmd}, args...)...)
	if errMini != nil {
		c.t.Errorf("error from miniredis: %s", errMini)
		return
	}

	// c.t.Logf("real:%q mini:%q", string(resReal), string(resMini))

	mini, err := proto.Parse(resMini)
	if err != nil {
		c.t.Errorf("parse error miniredis: %s", err)
		return
	}
	real, err := proto.Parse(resReal)
	if err != nil {
		c.t.Errorf("parse error realredis: %s", err)
		return
	}
	miniInt, ok := mini.(int)
	if !ok {
		c.t.Errorf("parse int error miniredis: %T found (%#v)", mini, mini)
		return
	}
	realInt, ok := real.(int)
	if !ok {
		c.t.Errorf("parse int error miniredis: %T found (%#v)", real, real)
		return
	}
	if math.Abs(float64(miniInt-realInt)) > float64(threshold) {
		c.t.Errorf("expected an approximated match (threshold is %d) want: %#v have: %#v", threshold, real, mini)
	}
}

// both must return an error, which much both Contain() the message.
func (c *client) Error(msg string, cmd string, args ...string) {
	c.t.Helper()

	resReal, errReal := c.real.Do(append([]string{cmd}, args...)...)
	if errReal != nil {
		c.t.Errorf("error from realredis: %s", errReal)
		return
	}
	resMini, errMini := c.mini.Do(append([]string{cmd}, args...)...)
	if errMini != nil {
		c.t.Errorf("error from miniredis: %s", errMini)
		return
	}

	mini, err := proto.ReadError(resMini)
	if err != nil {
		c.t.Logf("real:%q mini:%q", string(resReal), string(resMini))
		c.t.Errorf("parse error miniredis: %s", err)
		return
	}
	real, err := proto.ReadError(resReal)
	if err != nil {
		c.t.Errorf("parse error realredis: %s", err)
		return
	}

	if !strings.Contains(real, msg) {
		c.t.Errorf("expected (real)\n%q\nto contain %q", real, msg)
	}
	if !strings.Contains(mini, msg) {
		c.t.Errorf("expected (mini)\n%q\nto contain %q\nreal:\n%s", mini, msg, real)
	}
	// if real != mini {
	// c.t.Errorf("expected error:\n%q\ngot:\n%q", real, mini)
	// }
}

// both must return exactly the same error
func (c *client) ErrorTheSame(msg string, cmd string, args ...string) {
	c.t.Helper()

	resReal, errReal := c.real.Do(append([]string{cmd}, args...)...)
	if errReal != nil {
		c.t.Errorf("error from realredis: %s", errReal)
		return
	}
	resMini, errMini := c.mini.Do(append([]string{cmd}, args...)...)
	if errMini != nil {
		c.t.Errorf("error from miniredis: %s", errMini)
		return
	}

	mini, err := proto.ReadError(resMini)
	if err != nil {
		c.t.Logf("real:%q mini:%q", string(resReal), string(resMini))
		c.t.Errorf("parse error miniredis: %s", err)
		return
	}
	real, err := proto.ReadError(resReal)
	if err != nil {
		c.t.Errorf("parse error realredis: %s", err)
		return
	}

	if real != msg {
		c.t.Errorf("expected (real)\n%q\nto contain %q", real, msg)
	}
	if mini != msg {
		c.t.Errorf("expected (mini)\n%q\nto contain %q\nreal:\n%s", mini, msg, real)
	}
	// real == msg && mini == msg => real == mini, so we don't want to check it explicitly
}

// only receive a command, which can't be an error
func (c *client) Receive() {
	c.t.Helper()

	resReal, errReal := c.real.Read()
	if errReal != nil {
		c.t.Errorf("error from realredis: %s", errReal)
		return
	}
	resMini, errMini := c.mini.Read()
	if errMini != nil {
		c.t.Errorf("error from miniredis: %s", errMini)
		return
	}

	// c.t.Logf("real:%q mini:%q", string(resReal), string(resMini))

	if strings.HasPrefix(resReal, "-") {
		c.t.Errorf("error from realredis: %q", string(resReal))
	}
	if strings.HasPrefix(resMini, "-") {
		c.t.Errorf("error from miniredis: %q", string(resMini))
	}
}
