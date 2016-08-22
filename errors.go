package main

import "github.com/reconquest/ser-go"

func formatAbsPathError(path string, err error) error {
	return ser.Errorf(
		err, "can't get abs path for '%s'", path, err,
	)
}
