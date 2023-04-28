package environ

// Environ is a slice of strings representing the environment of a process.
type Environ []string

// Get retrieves the value of the environment variable named by the key.
// It returns the value, which will be empty if the variable is not present.
// To distinguish between an empty value and an unset value, use LookupEnv.
func (e Environ) Get(key string) string {
	v, _ := e.Lookup(key)
	return v
}

// Lookup retrieves the value of the environment variable named
// by the key. If the variable is present in the environment the
// value (which may be empty) is returned and the boolean is true.
// Otherwise the returned value will be empty and the boolean will
// be false.
func (e Environ) Lookup(key string) (string, bool) {
	for _, env := range e {
		if len(env) > len(key) && env[len(key)] == '=' && env[:len(key)] == key {
			return env[len(key)+1:], true
		}
	}
	return "", false
}
