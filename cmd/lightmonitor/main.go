package main

import (
	"context"
	"log"
	"os"

	"lightmonitor/internal/app"
	"lightmonitor/internal/config"
)

func main() {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	if err := app.Run(context.Background(), cfg); err != nil {
		log.Fatal(err)
	}
}
