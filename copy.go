package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
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
		return err
	}

	return nil
}

func copyDir(src string, dest string) (err error) {
	srcStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !srcStat.IsDir() {
		return fmt.Errorf("%s is not directory", src)
	}

	err = os.MkdirAll(dest, srcStat.Mode())
	if err != nil {
		return fmt.Errorf("can't mkdir %s: %s", dest)
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
			return fmt.Errorf(
				"can't copy  %s -> %s: %s", srcEntry, destEntry, err,
			)
		}
	}

	return nil
}
