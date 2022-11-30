package controllers

import (
	"fmt"
	"strings"
)

func CombineErrors(errs ...error) error {
	var errStrings []string
	for _, e := range errs {
		e := e
		errStrings = append(errStrings, e.Error())
	}

	switch len(errStrings) {
	case 0:
		return nil
	case 1:
		return fmt.Errorf(errStrings[0])
	default:
		return fmt.Errorf("multiple errors occurred: %s", strings.Join(errStrings, ", "))
	}
}
