package main

type storage interface {
	Init() error
	DeInit() error
	InitContainer(baseDir, container string) error
	DeInitContainer(container string) error
	InitImage(image string) error
	DestroyContainer(container string) error
	GetContainerRoot(container string) string
	Destroy() error
}
