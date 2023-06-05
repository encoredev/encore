package transport

// Transport is the interface for the transport layer which allows us to add
// and read metadata from the transport without having to know the underlying
// transport implementation.
type Transport interface {
	// SetMeta sets a key-value pair to the metadata of the transport.
	SetMeta(key string, value string)

	// ReadMeta reads a metadata key off the transport.
	// If there are multiple values for the key, the first value is returned.
	ReadMeta(key string) (value string, found bool)

	// ReadMetaValues reads all values for a metadata key off the transport.
	ReadMetaValues(key string) (values []string, found bool)
}
