package main

import (
	"fmt"
	"math/rand"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	cnt :=0
	x :=30000000
	for i :=0; i < x; i++ {
		n := rand.Int31n(1000)
		m := rand.Int31n(1000)
		if n == m {
			i--
			continue
		}
		n, m = sort(n, m)
		a := 1000 -n
		b := n - m
		c := m
		if trinagle(a, b ,c) {
			cnt++
		}
		//fmt.Println(m, n, a, b, c , cnt)
	}

	fmt.Println("= " , float64(cnt) * 100/ float64(x))
}


func sort(a, b int32) (int32, int32) {
	if a > b {
		return a, b
	}
	return b, a
}

func trinagle(a, b, c int32) bool {
	if a == 0 || b == 0 || c == 0 {
		return false
	}
	return !( a > 500 || b > 500 || c > 500 )
}