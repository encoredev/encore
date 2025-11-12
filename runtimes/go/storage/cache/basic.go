package cache

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

// NewStringKeyspace creates a keyspace that stores string values in the given cluster.
//
// The type parameter K specifies the key type, which can either be a
// named struct type or a basic type (string, int, etc).
func NewStringKeyspace[K any](cluster *Cluster, cfg KeyspaceConfig) *StringKeyspace[K] {
	fromRedis := func(val string) (string, error) { return val, nil }
	toRedis := func(val string) (any, error) { return val, nil }

	return &StringKeyspace[K]{
		&basicKeyspace[K, string]{
			newClient[K, string](cluster, cfg, fromRedis, toRedis),
		},
	}
}

// StringKeyspace represents a set of cache keys that hold string values.
type StringKeyspace[K any] struct {
	*basicKeyspace[K, string]
}

// Get gets the value stored at key.
// If the key does not exist, it returns an error matching Miss.
//
// See https://redis.io/commands/get/ for more information.
func (s *StringKeyspace[K]) Get(ctx context.Context, key K) (string, error) {
	return s.basicKeyspace.Get(ctx, key)
}

// MultiGet gets the values stored at multiple keys.
// For each key, the result contains an Err field indicating success or failure.
// If Err is nil, Value contains the cached value.
// If Err matches Miss, the key was not found.
//
// See https://redis.io/commands/mget/ for more information.
func (s *StringKeyspace[K]) MultiGet(ctx context.Context, keys ...K) ([]Result[string], error) {
	return s.basicKeyspace.MultiGet(ctx, keys...)
}

// Set updates the value stored at key to val.
//
// See https://redis.io/commands/set/ for more information.
func (s *StringKeyspace[K]) Set(ctx context.Context, key K, val string) error {
	return s.basicKeyspace.Set(ctx, key, val)
}

// SetIfNotExists sets the value stored at key to val, but only if the key does not exist beforehand.
// If the key already exists, it reports an error matching KeyExists.
//
// See https://redis.io/commands/setnx/ for more information.
func (s *StringKeyspace[K]) SetIfNotExists(ctx context.Context, key K, val string) error {
	return s.basicKeyspace.SetIfNotExists(ctx, key, val)
}

// Replace replaces the existing value stored at key to val.
// If the key does not already exist, it reports an error matching Miss.
//
// See https://redis.io/commands/set/ for more information.
func (s *StringKeyspace[K]) Replace(ctx context.Context, key K, val string) error {
	return s.basicKeyspace.Replace(ctx, key, val)
}

// GetAndSet updates the value of key to val and returns the previously stored value.
// If the key does not already exist, it sets it and returns "", nil.
//
// See https://redis.io/commands/getset/ for more information.
func (s *StringKeyspace[K]) GetAndSet(ctx context.Context, key K, val string) (oldVal string, err error) {
	return s.basicKeyspace.GetAndSet(ctx, key, val)
}

// GetAndDelete deletes the key and returns the previously stored value.
// If the key does not already exist, it does nothing and returns "", nil.
//
// See https://redis.io/commands/getdel/ for more information.
func (s *StringKeyspace[K]) GetAndDelete(ctx context.Context, key K) (oldVal string, err error) {
	return s.basicKeyspace.GetAndDelete(ctx, key)
}

// Delete deletes the specified keys.
//
// If a key does not exist it is ignored.
//
// It reports the number of keys that were deleted.
//
// See https://redis.io/commands/del/ for more information.
func (s *StringKeyspace[K]) Delete(ctx context.Context, keys ...K) (deleted int, err error) {
	return s.client.Delete(ctx, keys...)
}

// With returns a reference to the same keyspace but with customized write options.
// The primary use case is for overriding the expiration time for certain cache operations.
//
// It is intended to be used with method chaining:
//
//	myKeyspace.With(cache.ExpireIn(3 * time.Second)).Set(...)
func (k *StringKeyspace[K]) With(opts ...WriteOption) *StringKeyspace[K] {
	return &StringKeyspace[K]{k.with(opts)}
}

// Append appends to the string with the given key.
//
// If the key does not exist it is first created and set as the empty string,
// causing Append to behave like Set.
//
// It returns the new string length.
//
// See https://redis.io/commands/append/ for more information.
func (s *StringKeyspace[K]) Append(ctx context.Context, key K, val string) (newLen int64, err error) {
	const op = "append"
	k, err := s.key(key, op)
	endTrace := s.doTrace(op, true, k)
	defer func() { endTrace(err) }()
	if err != nil {
		return 0, err
	}

	res, err := do(s.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.Append(ctx, k, val)
	}).Result()
	err = toErr(err, op, k)
	return res, err
}

// GetRange returns a substring of the string value stored in key.
//
// The from and to values are zero-based indices, but unlike Go slicing
// the 'to' value is inclusive.
//
// Negative values can be used in order to provide an offset starting
// from the end of the string, so -1 means the last character
// and -len(str) the first character, and so forth.
//
// If the string does not exist it returns the empty string.
//
// See https://redis.io/commands/setrange/ for more information.
func (s *StringKeyspace[K]) GetRange(ctx context.Context, key K, from, to int64) (val string, err error) {
	const op = "get range"
	k, err := s.key(key, op)
	endTrace := s.doTrace(op, false, k)
	defer func() { endTrace(err) }()
	if err != nil {
		return "", err
	}

	res, err := s.client.redis.GetRange(ctx, k, from, to).Result()
	err = toErr(err, op, k)
	return res, err
}

// SetRange overwrites part of the string stored at key, starting at
// the zero-based offset and for the entire length of val, extending
// the string if necessary to make room for val.
//
// If the offset is larger than the current string length stored at key,
// the string is first padded with zero-bytes to make offset fit.
//
// Non-existing keys are considered as empty strings.
//
// See https://redis.io/commands/setrange/ for more information.
func (s *StringKeyspace[K]) SetRange(ctx context.Context, key K, offset int64, val string) (newLen int64, err error) {
	const op = "set range"
	k, err := s.key(key, op)
	endTrace := s.doTrace(op, true, k)
	defer func() { endTrace(err) }()
	if err != nil {
		return 0, err
	}

	res, err := do[K, string, *redis.IntCmd](s.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.SetRange(ctx, k, offset, val)
	}).Result()
	err = toErr(err, op, k)
	return res, err
}

// Len reports the length of the string value stored at key.
//
// Non-existing keys are considered as empty strings.
//
// See https://redis.io/commands/strlen/ for more information.
func (s *StringKeyspace[K]) Len(ctx context.Context, key K) (length int64, err error) {
	const op = "len"
	k, err := s.key(key, op)
	endTrace := s.doTrace(op, false, k)
	defer func() { endTrace(err) }()
	if err != nil {
		return 0, err
	}

	res, err := s.client.redis.StrLen(ctx, k).Result()
	err = toErr(err, op, k)
	return res, err
}

// NewIntKeyspace creates a keyspace that stores int64 values in the given cluster.
//
// The type parameter K specifies the key type, which can either be a
// named struct type or a basic type (string, int, etc).
func NewIntKeyspace[K any](cluster *Cluster, cfg KeyspaceConfig) *IntKeyspace[K] {
	fromRedis := func(val string) (int64, error) { return strconv.ParseInt(val, 10, 64) }
	toRedis := func(val int64) (any, error) { return val, nil }

	return &IntKeyspace[K]{
		&basicKeyspace[K, int64]{
			newClient[K, int64](cluster, cfg, fromRedis, toRedis),
		},
	}
}

// IntKeyspace is a cache keyspace that stores int64 values.
type IntKeyspace[K any] struct {
	*basicKeyspace[K, int64]
}

// With returns a reference to the same keyspace but with customized write options.
// The primary use case is for overriding the expiration time for certain cache operations.
//
// It is intended to be used with method chaining:
//
//	myKeyspace.With(cache.ExpireIn(3 * time.Second)).Set(...)
func (k *IntKeyspace[K]) With(opts ...WriteOption) *IntKeyspace[K] {
	return &IntKeyspace[K]{k.basicKeyspace.with(opts)}
}

// Get gets the value stored at key.
// If the key does not exist, it returns an error matching Miss.
//
// See https://redis.io/commands/get/ for more information.
func (s *IntKeyspace[K]) Get(ctx context.Context, key K) (int64, error) {
	return s.basicKeyspace.Get(ctx, key)
}

// MultiGet gets the values stored at multiple keys.
// For each key, the result contains an Err field indicating success or failure.
// If Err is nil, Value contains the cached value.
// If Err matches Miss, the key was not found.
//
// See https://redis.io/commands/mget/ for more information.
func (s *IntKeyspace[K]) MultiGet(ctx context.Context, keys ...K) ([]Result[int64], error) {
	return s.basicKeyspace.MultiGet(ctx, keys...)
}

// Set updates the value stored at key to val.
//
// See https://redis.io/commands/set/ for more information.
func (s *IntKeyspace[K]) Set(ctx context.Context, key K, val int64) error {
	return s.basicKeyspace.Set(ctx, key, val)
}

// SetIfNotExists sets the value stored at key to val, but only if the key does not exist beforehand.
// If the key already exists, it reports an error matching KeyExists.
//
// See https://redis.io/commands/setnx/ for more information.
func (s *IntKeyspace[K]) SetIfNotExists(ctx context.Context, key K, val int64) error {
	return s.basicKeyspace.SetIfNotExists(ctx, key, val)
}

// Replace replaces the existing value stored at key to val.
// If the key does not already exist, it reports an error matching Miss.
//
// See https://redis.io/commands/set/ for more information.
func (s *IntKeyspace[K]) Replace(ctx context.Context, key K, val int64) error {
	return s.basicKeyspace.Replace(ctx, key, val)
}

// GetAndSet updates the value of key to val and returns the previously stored value.
// If the key does not already exist, it sets it and returns 0, nil.
//
// See https://redis.io/commands/getset/ for more information.
func (s *IntKeyspace[K]) GetAndSet(ctx context.Context, key K, val int64) (oldVal int64, err error) {
	return s.basicKeyspace.GetAndSet(ctx, key, val)
}

// GetAndDelete deletes the key and returns the previously stored value.
// If the key does not already exist, it does nothing and returns 0, nil.
//
// See https://redis.io/commands/getdel/ for more information.
func (s *IntKeyspace[K]) GetAndDelete(ctx context.Context, key K) (oldVal int64, err error) {
	return s.basicKeyspace.GetAndDelete(ctx, key)
}

// Delete deletes the specified keys.
//
// If a key does not exist it is ignored.
//
// It reports the number of keys that were deleted.
//
// See https://redis.io/commands/del/ for more information.
func (s *IntKeyspace[K]) Delete(ctx context.Context, keys ...K) (deleted int, err error) {
	return s.client.Delete(ctx, keys...)
}

// Increment increments the number stored in key by delta,
// and returns the new value.
//
// If the key does not exist it is first created with a value of 0
// before incrementing.
//
// Negative values can be used to decrease the value,
// but typically you want to use the Decrement method for that.
//
// See https://redis.io/commands/incrby/ for more information.
func (s *IntKeyspace[K]) Increment(ctx context.Context, key K, delta int64) (newVal int64, err error) {
	const op = "increment"
	k, err := s.key(key, op)
	endTrace := s.doTrace(op, true, k)
	defer func() { endTrace(err) }()
	if err != nil {
		return 0, err
	}

	res, err := do(s.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.IncrBy(ctx, k, delta)
	}).Result()
	err = toErr(err, op, k)
	return res, err
}

// Decrement decrements the number stored in key by delta,
// and returns the new value.
//
// If the key does not exist it is first created with a value of 0
// before decrementing.
//
// Negative values can be used to increase the value,
// but typically you want to use the Increment method for that.
//
// See https://redis.io/commands/decrby/ for more information.
func (s *IntKeyspace[K]) Decrement(ctx context.Context, key K, delta int64) (newVal int64, err error) {
	const op = "decrement"
	k, err := s.key(key, op)
	endTrace := s.doTrace(op, true, k)
	defer func() { endTrace(err) }()
	if err != nil {
		return 0, err
	}

	res, err := do(s.client, ctx, k, func(c cmdable) *redis.IntCmd {
		return c.DecrBy(ctx, k, delta)
	}).Result()

	err = toErr(err, op, k)
	return res, err
}

// NewFloatKeyspace creates a keyspace that stores float64 values in the given cluster.
//
// The type parameter K specifies the key type, which can either be a
// named struct type or a basic type (string, int, etc).
func NewFloatKeyspace[K any](cluster *Cluster, cfg KeyspaceConfig) *FloatKeyspace[K] {
	fromRedis := func(val string) (float64, error) { return strconv.ParseFloat(val, 64) }
	toRedis := func(val float64) (any, error) { return val, nil }

	return &FloatKeyspace[K]{
		&basicKeyspace[K, float64]{
			newClient[K, float64](cluster, cfg, fromRedis, toRedis),
		},
	}
}

// FloatKeyspace is a cache keyspace that stores float64 values.
type FloatKeyspace[K any] struct {
	*basicKeyspace[K, float64]
}

// With returns a reference to the same keyspace but with customized write options.
// The primary use case is for overriding the expiration time for certain cache operations.
//
// It is intended to be used with method chaining:
//
//	myKeyspace.With(cache.ExpireIn(3 * time.Second)).Set(...)
func (k *FloatKeyspace[K]) With(opts ...WriteOption) *FloatKeyspace[K] {
	return &FloatKeyspace[K]{k.basicKeyspace.with(opts)}
}

// Get gets the value stored at key.
// If the key does not exist, it returns an error matching Miss.
//
// See https://redis.io/commands/get/ for more information.
func (s *FloatKeyspace[K]) Get(ctx context.Context, key K) (float64, error) {
	return s.basicKeyspace.Get(ctx, key)
}

// MultiGet gets the values stored at multiple keys.
// For each key, the result contains an Err field indicating success or failure.
// If Err is nil, Value contains the cached value.
// If Err matches Miss, the key was not found.
//
// See https://redis.io/commands/mget/ for more information.
func (s *FloatKeyspace[K]) MultiGet(ctx context.Context, keys ...K) ([]Result[float64], error) {
	return s.basicKeyspace.MultiGet(ctx, keys...)
}

// Set updates the value stored at key to val.
//
// See https://redis.io/commands/set/ for more information.
func (s *FloatKeyspace[K]) Set(ctx context.Context, key K, val float64) error {
	return s.basicKeyspace.Set(ctx, key, val)
}

// SetIfNotExists sets the value stored at key to val, but only if the key does not exist beforehand.
// If the key already exists, it reports an error matching KeyExists.
//
// See https://redis.io/commands/setnx/ for more information.
func (s *FloatKeyspace[K]) SetIfNotExists(ctx context.Context, key K, val float64) error {
	return s.basicKeyspace.SetIfNotExists(ctx, key, val)
}

// Replace replaces the existing value stored at key to val.
// If the key does not already exist, it reports an error matching Miss.
//
// See https://redis.io/commands/set/ for more information.
func (s *FloatKeyspace[K]) Replace(ctx context.Context, key K, val float64) error {
	return s.basicKeyspace.Replace(ctx, key, val)
}

// GetAndSet updates the value of key to val and returns the previously stored value.
// If the key does not already exist, it sets it and returns 0, nil.
//
// See https://redis.io/commands/getset/ for more information.
func (s *FloatKeyspace[K]) GetAndSet(ctx context.Context, key K, val float64) (oldVal float64, err error) {
	return s.basicKeyspace.GetAndSet(ctx, key, val)
}

// GetAndDelete deletes the key and returns the previously stored value.
// If the key does not already exist, it does nothing and returns 0, nil.
//
// See https://redis.io/commands/getdel/ for more information.
func (s *FloatKeyspace[K]) GetAndDelete(ctx context.Context, key K) (oldVal float64, err error) {
	return s.basicKeyspace.GetAndDelete(ctx, key)
}

// Delete deletes the specified keys.
//
// If a key does not exist it is ignored.
//
// It reports the number of keys that were deleted.
//
// See https://redis.io/commands/del/ for more information.
func (s *FloatKeyspace[K]) Delete(ctx context.Context, keys ...K) (deleted int, err error) {
	return s.client.Delete(ctx, keys...)
}

// Increment increments the number stored in key by delta,
// and returns the new value.
//
// If the key does not exist it is first created with a value of 0
// before incrementing.
//
// Negative values can be used to decrease the value,
// but typically you want to use the Decrement method for that.
//
// See https://redis.io/commands/incrbyfloat/ for more information.
func (s *FloatKeyspace[K]) Increment(ctx context.Context, key K, delta float64) (newVal float64, err error) {
	const op = "increment"
	k, err := s.key(key, op)
	endTrace := s.doTrace(op, true, k)
	defer func() { endTrace(err) }()
	if err != nil {
		return 0, err
	}

	res, err := do[K, float64, *redis.FloatCmd](s.client, ctx, k, func(c cmdable) *redis.FloatCmd {
		return c.IncrByFloat(ctx, k, delta)
	}).Result()
	err = toErr(err, op, k)
	return res, err
}

// Decrement decrements the number stored in key by delta,
// and returns the new value.
//
// If the key does not exist it is first created with a value of 0
// before decrementing.
//
// Negative values can be used to increase the value,
// but typically you want to use the Increment method for that.
//
// See https://redis.io/commands/incrbyfloat/ for more information.
func (s *FloatKeyspace[K]) Decrement(ctx context.Context, key K, delta float64) (newVal float64, err error) {
	const op = "decrement"
	k, err := s.key(key, op)
	endTrace := s.doTrace(op, true, k)
	defer func() { endTrace(err) }()
	if err != nil {
		return 0, err
	}

	res, err := do[K, float64, *redis.FloatCmd](s.client, ctx, k, func(c cmdable) *redis.FloatCmd {
		return c.IncrByFloat(ctx, k, -delta)
	}).Result()
	err = toErr(err, op, k)
	return res, err
}

type basicKeyspace[K, V any] struct {
	*client[K, V]
}

func (s *basicKeyspace[K, V]) with(opts []WriteOption) *basicKeyspace[K, V] {
	return &basicKeyspace[K, V]{s.client.with(opts)}
}

func (s *basicKeyspace[K, V]) Get(ctx context.Context, key K) (val V, err error) {
	const op = "get"
	k, err := s.key(key, op)
	endTrace := s.doTrace(op, false, k)
	defer func() { endTrace(err) }()
	if err != nil {
		return val, err
	}

	res, err := s.redis.Get(ctx, k).Result()
	if err == nil {
		val, err = s.fromRedis(res)
	}
	err = toErr(err, op, k)
	return val, err
}

func (s *basicKeyspace[K, V]) MultiGet(ctx context.Context, keys ...K) ([]Result[V], error) {
	const op = "multi get"
	ks, err := s.keys(keys, op)
	endTrace := s.doTrace(op, false, ks...)
	defer func() { endTrace(err) }()
	if err != nil {
		return nil, err
	}
	var firstKey string
	if len(ks) > 0 {
		firstKey = ks[0]
	}
	res, err := s.redis.MGet(ctx, ks...).Result()
	if err != nil {
		return nil, toErr(err, op, firstKey)
	}
	results := make([]Result[V], 0, len(res))
	for i, r := range res {
		if r == nil {
			results = append(results, Result[V]{Err: toErr(Miss, op, ks[i])})
			continue
		}
		strVal, ok := r.(string)
		if !ok {
			results = append(results, Result[V]{Err: toErr(errors.New("invalid redis value type"), op, ks[i])})
			continue
		}
		val, fromRedisErr := s.fromRedis(strVal)
		if fromRedisErr != nil {
			results = append(results, Result[V]{Err: toErr(fromRedisErr, op, ks[i])})
			continue
		}
		results = append(results, Result[V]{Value: val})
	}
	return results, nil
}

func (s *basicKeyspace[K, V]) Set(ctx context.Context, key K, val V) error {
	_, _, err := s.set(ctx, key, val, 0, "set")
	return err
}

func (s *basicKeyspace[K, V]) SetIfNotExists(ctx context.Context, key K, val V) error {
	const op = "set if not exists"
	_, _, err := s.set(ctx, key, val, setNX, op)
	return err
}

func (s *basicKeyspace[K, V]) Replace(ctx context.Context, key K, val V) error {
	_, _, err := s.set(ctx, key, val, setXX, "replace")
	return err
}

func (s *basicKeyspace[K, V]) GetAndSet(ctx context.Context, key K, val V) (prev V, err error) {
	const op = "get and set"
	res, k, err := s.set(ctx, key, val, setGet, op)
	if err == nil {
		val, err = s.fromRedis(res)
		err = toErr(err, op, k)
	}
	return val, err
}

func (s *basicKeyspace[K, V]) GetAndDelete(ctx context.Context, key K) (val V, err error) {
	const op = "get and delete"
	k, err := s.key(key, op)
	endTrace := s.doTrace(op, true, k)
	defer func() { endTrace(err) }()
	if err != nil {
		return val, err
	}

	// When deleting we don't need to deal with expiry
	res, err := s.redis.GetDel(ctx, k).Result()
	if err == nil {
		val, err = s.fromRedis(res)
	}
	err = toErr(err, op, k)
	return val, err
}

func (s *client[K, V]) Delete(ctx context.Context, keys ...K) (deleted int, err error) {
	const op = "delete"
	ks, err := s.keys(keys, op)
	endTrace := s.doTrace(op, true, ks...)
	defer func() { endTrace(err) }()
	if err != nil {
		return 0, err
	}

	var firstKey string
	if len(ks) > 0 {
		firstKey = ks[0]
	}

	// When deleting we don't need to deal with expiry
	res, err := s.redis.Del(ctx, ks...).Result()
	err = toErr(err, op, firstKey)
	return int(res), err
}

type setFlag uint8

const (
	setGet setFlag = 1 << iota
	setNX
	setXX
)

func (s *basicKeyspace[K, V]) set(ctx context.Context, key K, val V, flag setFlag, op string) (strVal, k string, err error) {
	k, err = s.key(key, op)
	if err != nil {
		return "", "", err
	}

	endTrace := s.doTrace(op, true, k)
	defer func() { endTrace(err) }()

	get := (flag & setGet) == setGet
	nx := (flag & setNX) == setNX
	xx := (flag & setXX) == setXX

	if nx {
		// If this is a setNX, convert Miss to KeyExists.
		defer func() {
			if errors.Is(err, Miss) {
				err = toErr(KeyExists, op, k)
			}
		}()
	}

	redisVal, err := s.toRedis(val)
	if err != nil {
		return "", k, toErr(err, op, k)
	}

	args := make([]any, 3, 7)
	args[0] = "set"
	args[1] = k
	args[2] = redisVal
	if nx {
		args = append(args, "nx")
	} else if xx {
		args = append(args, "xx")
	}
	if get {
		args = append(args, "get")
	}

	now := time.Now()
	exp := s.expiry(now)
	switch exp {
	case neverExpire:
		// do nothing; default Redis behavior
	case keepTTL:
		args = append(args, "keepttl")
	default:
		dur := exp.Sub(now)
		if dur < 0 {
			// The expiry is in the past; use a very old unix timestamp to
			// delete the key immediately. Note that we can't use timestamp 0
			// or else [Mini]redis complains.
			args = append(args, "exat", 1)
		} else {
			if usePreciseDur(dur) {
				args = append(args, "px", int64(dur/time.Millisecond))
			} else {
				args = append(args, "ex", int64(dur/time.Second))
			}
		}
	}

	if get {
		cmd := redis.NewStringCmd(ctx, args...)
		_ = s.redis.Process(ctx, cmd)
		res, err := cmd.Result()
		err = toErr(err, op, k)
		return res, k, err
	}

	cmd := redis.NewStatusCmd(ctx, args...)
	_ = s.redis.Process(ctx, cmd)
	return "", k, toErr(cmd.Err(), op, k)
}

func usePreciseDur(dur time.Duration) bool {
	return dur < time.Second || dur%time.Second != 0
}
