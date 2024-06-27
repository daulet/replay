package main

import "log"

func main() {
	if err := serve(8080); err != nil {
		log.Fatal(err)
	}
}
