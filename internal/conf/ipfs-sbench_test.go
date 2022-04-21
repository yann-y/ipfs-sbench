package conf

import (
	"fmt"
	"testing"
)

func init() {
	testing.Init()
}
func Test_aaa(t *testing.T) {
	ch := make(chan int8, 10)
	switch <-ch {
	case 0:
		fmt.Println(0)
	case 1:
		fmt.Println(1)
	default:
		fmt.Println("hjhjkhk")
	}
	fmt.Println("yes")
}
