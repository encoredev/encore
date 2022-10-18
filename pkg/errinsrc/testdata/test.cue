package test_cue

// This is a sample file
blah: {
	foo: bool
	bar: int | *3
}

// Let's set foo to true
blah: foo: true

// If foo then bar is 12
if blah.foo {
	blah: bar: 12
}
