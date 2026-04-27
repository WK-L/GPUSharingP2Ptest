package main

import (
	"log"

	"gpusharingp2ptest/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
