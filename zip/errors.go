package zip

import "fmt"

type ZipError struct {
	Verb, Path string
	Err        error
}

func (e *ZipError) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("%s: %v", e.Verb, e.Err)
	} else {
		return fmt.Sprintf("%s %s: %v", e.Verb, e.Path, e.Err)
	}
}

func (e *ZipError) Unwrap() error {
	return e.Err
}
