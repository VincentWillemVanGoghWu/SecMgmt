package main

import (
	"os"
	"path/filepath"

	"secmgmt_go/internal/bootstrap"
)

func main() {
	rootDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	app, err := bootstrap.Build(filepath.Clean(rootDir))
	if err != nil {
		panic(err)
	}
	if app.Close != nil {
		app.Close()
	}
}
