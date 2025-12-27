package main

import (
	"context"
	"os"

	"school-with-ai/components"
)

func main() {
	component := components.Hello("John")
	component.Render(context.Background(), os.Stdout)
}
