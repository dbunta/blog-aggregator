package main

import (
	"fmt"

	config "github.com/dbunta/blog-aggregator/internal/config"
)

func main() {
	c, err := config.Read()
	if err != nil {
		fmt.Errorf("Error getting config: %w", err)
	}
	c.SetUser("lane")
	c, err = config.Read()
	fmt.Print(c)
}
