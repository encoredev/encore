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
