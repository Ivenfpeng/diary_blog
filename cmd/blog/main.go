package main

import (
	"fmt"
	"os"

	"github.com/Ivenfpeng/diary_blog/internal/config"
)

func main() {
	if _, err := config.Load(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("diary-blog bootstrap ready")
}
