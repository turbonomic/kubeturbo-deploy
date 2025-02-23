package utils

// Utility function to return a pointer to the function argument.
// Useful for creating in-line primitive pointers, e.g. AsPtr(123), AsPtr(true)
func AsPtr[T any](t T) *T {
	return &t
}
