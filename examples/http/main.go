package main

import (
	"context"
	"log"
)

func main() {
	ctx := context.Background()

	go func() {
		if err := ServeDependency(ctx, 8081); err != nil {
			log.Fatal(err)
		}
	}()

	if err := ServeApplication(ctx, 8080, "localhost:8081"); err != nil {
		log.Fatal(err)
	}
}
