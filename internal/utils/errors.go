package utils

// a no-arg function which returns an error
type ErrorFn func() error

// This is a convenience function to facilitate error handling
// Use this when you need to execute a series of failable functions
// and return as soon as the first error occurs
func ReturnOnError(fns ...ErrorFn) error {
	for _, f := range fns {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}
