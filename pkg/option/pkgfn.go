package option

// Contains returns true if the option is present and matches the given value
func Contains[T comparable](option Option[T], matches T) bool {
	if option.present {
		return option.value == matches
	}
	return false
}

// Map returns an Option with the value mapped by the given function if present, otherwise returns None
func Map[T, R any](option Option[T], f func(T) R) Option[R] {
	if option.present {
		return Some(f(option.value))
	}
	return None[R]()
}

// FlatMap returns an Option with the value mapped by the given function if present, otherwise returns None
func FlatMap[T, R any](option Option[T], f func(T) Option[R]) Option[R] {
	if option.present {
		return f(option.value)
	}
	return None[R]()
}

// Fold returns the result of f applied to the value if present, otherwise returns the defaultValue
func Fold[T, R any](option Option[T], defaultValue R, f func(T) R) R {
	if option.present {
		return f(option.value)
	}
	return defaultValue
}

// FoldLeft applies the binary operator f to the value if present, otherwise returns the zero value
func FoldLeft[T, R any](option Option[T], zero R, f func(accum R, value T) R) R {
	if option.present {
		return f(zero, option.value)
	}
	return zero
}

// Equal returns true if both Options are equal.
func Equal[T comparable](a, b Option[T]) bool {
	if a.present != b.present {
		return false
	}
	if !a.present {
		return true
	}
	return a.value == b.value
}
