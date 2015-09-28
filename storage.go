package main

type storage interface {
	Init() error
	Merge(base, data, target string) error
	Destroy() error
}
