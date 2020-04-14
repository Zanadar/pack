package image

import (
	"fmt"
	"math/rand"
)

func RandomName(fmtString string, n int) string {
	return fmt.Sprintf(fmtString, randString(n))
}

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a' + byte(rand.Intn(26))
	}
	return string(b)
}
