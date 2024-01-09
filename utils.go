package main

import (
	"math/rand"
	"os"
	"path/filepath"
)

func PickOne[T any](population []T) T {
	randomIndex := rand.Intn(len(population))
	return population[randomIndex]
}

func Shuffle[T any](slice []T) {
	for i := len(slice) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}

func getExecutableDirectory() string {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return filepath.Dir(ex)
}
