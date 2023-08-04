semgrep-go
==========

> *Encore Note*
> 
> We've copied in the rules from https://github.com/dgryski/semgrep-go on 2023-07-27
> However delete some rules that we don't want to run in our codebase and updated
> some other rules.

This repo holds patterns for finding odd Go code.

The rules engines currently supported:

* [semgrep](https://semgrep.dev/)
* [ruleguard](https://github.com/quasilyte/go-ruleguard)

I'll accept [comby](https://comby.dev) patterns if you can't get them to work with either semgrep or ruleguard.

To run a single semgrep rule:

```
$ semgrep -f path/to/semgrep-go/rule.yml .
```

To run all semgrep rules:

```
$ semgrep -f path/to/semgrep-go/ .
```

To run all the ruleguard rules:

```
$ ruleguard -c=0 -rules path/to/semgrep-go/ruleguard.rules.go ./...
```


Semgrep checks:
* badexponentiation: check for `2^x` and `10^x` which look like exponentiation
* badnilguard: check for poorly formed nil guards
* errtodo: check for TODOs in error handling code
* hmac-bytes: check for using bytes.Equal() with HMACs
* hostport: check for using fmt.Sprintf() instead of net.JoinHostPort()
* mathbits: check for places you should use math/bits instead
* mail-address: check for using fmt.Sprintf() instead of net/mail.Address.String()
* oddbitwise: check for odd bit-wise expressions
* oddcompare: check for odd comparisons
* oddcompound: check for odd compound += or -= expressions
* oddifsequence: check for an odd sequence of ifs
* oddmathbits: check for odd uses of math/bits
* parseint-downcast: check for places a parsed 64-bit int is downcast to 32-bits
* returnnil: check for odd nil-returns
* sprinterr: check for fmt.Sprint(err) instead of err.Error()
* joinpath: check for using strings.Join() to construct paths
* readfull: check for extra length check for io.ReadFull()
* nilerr: returning a nil err instead of a nil value
* errclosed: check for call strings.Contains() to detect net.ErrClosed
* hmac-hash: check for bad hash.New passed to hmac.New()
* readeof: check for ignoring io.EOF as a successful read
* writestring: check for using io.WriteString(w, string(b))
* wronglock: find incorrect lock/unlock pairs for rwmutex
* contexttodo: find context.TODO() usage and suggest to change it
* close-sql-query-rows: find places database/sql.Rows instance isn't Close()d
* unixnano: check for time.Time comparisons using UnixNano()
* timeafter: leaky use of time.After()
* contextCancelable: checks for cancelable contexts not systematically canceled

Ruleguard checks are in ruleguard.rules.go.
* unconvert: check for unnecessary conversions
* timeeq: check for using == and != with time.Time values
* errnoterror: check for variables called `err` which are not the error type
* ifbodythenbody: check for if statements with identical if and else bodies
* subtractnoteq: check for x-y==0 instead of x==y
* selfassign: check for variable self-assignments
* oddnestedif: check for odd patterns of nested-ifs.
* oddbitwise: check for odd bitwise expressions
* ifreturn: check for off if/return sequences
* oddifsequence: check for if sequences
* nestedifsequence: check for odd nested if sequences
* identicalassignments:  check for `x = y ; y = x` pairs.
* oddcompoundop: check for odd compound operations
* constswitch: check for switch statements with expressions
* oddcomparisons: check for odd comparisons
* oddmathbits: check for odd uses of math/bits
* floateq: check for exact comparisons of floating point values
* badexponent: check for `2^x` and `10^x` , which look like exponentiation
* floatloop: check for using floats as loop counters
* urlredacted: check for logging urls without calling url.Redacted()
* sprinterr: check for calling fmt.Sprint(err) instead of err.Error()
* largeloopcopy: check for large value copies in loops
* joinpath: check for using strings.Join() to construct paths
* readfull: check for extra length check for io.ReadFull()
* nilerr: returning an nil error instead of a nil value
* errnetclosed: check for call strings.Contains() to detect net.ErrClosed
* hmac-hash: check for bad hash.New passed to hmac.New()
* readeof: check for ignoring io.EOF as a successful read
* writestring: check for using io.WriteString(w, string(b)) when b is []byte
* badlock: find incorrect lock/unlock pairs for rwmutex
* contexttodo: find context.TODO() usage and suggest to change it
_

*Find this useful? [Buy me a coffee!](https://www.buymeacoffee.com/dgryski)*
