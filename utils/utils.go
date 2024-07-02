package utils

import (
	"fmt"
	"math/rand"
	"os"
	"time"
)

func GetEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ShuffleHeaderOrder 随机打乱给定的字符串切片
func ShuffleHeaderOrder(order []string) {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(order), func(i, j int) {
		order[i], order[j] = order[j], order[i]
	})
}
func GenerateRandomViewportWidth() string {
	rand.Seed(time.Now().UnixNano())
	width := rand.Intn(1920-300+1) + 300 // generates a random number between 300 and 1920
	return fmt.Sprintf("%d", width)
}
