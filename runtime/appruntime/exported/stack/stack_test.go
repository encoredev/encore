package stack

import (
	"testing"
)

func BenchmarkBuild(b *testing.B) {
	sum := 0
	for i := 0; i < b.N; i++ {
		n := __encore_foo()
		sum += n
	}
	b.Log(sum)
}

func __encore_foo() int {
	return userCode()
}

func userCode() int {
	s := Build(2)
	return len(s.Frames)
}
