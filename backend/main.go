package main

import (
	"log/slog"
)

func main() {
	slog.Info("Starting the server")
	StartApiServer()
}
