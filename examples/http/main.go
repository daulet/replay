package main

import (
	"context"
	"log"
)

func main() {
	if err := ServeDependency(context.Background(), 8080); err != nil {
		log.Fatal(err)
	}
}
