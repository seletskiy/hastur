package main

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/reconquest/ser-go"
)

func copyFile(src string, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return err
	}

	stat, err := os.Stat(src)
	if err != nil {
		return err
	}

	err = os.Chmod(dest, stat.Mode())
	if err != nil {
		return ser.Errorf(
			err, "can't change file mode: %s", dest,
		)
	}

	return nil
}

func copyDir(src string, dest string) (err error) {
	srcStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !srcStat.IsDir() {
		return ser.Errorf(
			err, "%s is not directory", src,
		)
	}

	err = os.MkdirAll(dest, srcStat.Mode())
	if err != nil {
		return ser.Errorf(
			err, "can't mkdir %s", dest,
		)
	}

	entries, err := ioutil.ReadDir(src)

	for _, entry := range entries {
		srcEntry := filepath.Join(src, entry.Name())
		destEntry := filepath.Join(dest, entry.Name())

		if entry.IsDir() {
			err = copyDir(srcEntry, destEntry)
		} else {
			err = copyFile(srcEntry, destEntry)
		}

		if err != nil {
			return ser.Errorf(
				err,
				"can't copy %s -> %s", srcEntry, destEntry,
			)
		}
	}

	return nil
}
