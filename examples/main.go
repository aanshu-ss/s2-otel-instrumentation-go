// package main

// import (
// 	"fmt"

// 	"github.com/yourusername/myutils/mathutils"
// 	"github.com/yourusername/myutils/stringutils"
// )

// func main() {
// 	fmt.Println("=== String Utils Demo ===")

// 	text := "hello world"
// 	fmt.Printf("Original: %s\n", text)
// 	fmt.Printf("Reversed: %s\n", stringutils.Reverse(text))
// 	fmt.Printf("Capitalized: %s\n", stringutils.Capitalize(text))
// 	fmt.Printf("Word count: %d\n", stringutils.WordCount(text))

// 	palindrome := "racecar"
// 	fmt.Printf("'%s' is palindrome: %v\n", palindrome, stringutils.IsPalindrome(palindrome))

// 	fmt.Println("\n=== Math Utils Demo ===")

// 	a, b := 15, 25
// 	fmt.Printf("Max(%d, %d) = %d\n", a, b, mathutils.Max(a, b))
// 	fmt.Printf("Min(%d, %d) = %d\n", a, b, mathutils.Min(a, b))

// 	n := 5
// 	fmt.Printf("Factorial(%d) = %d\n", n, mathutils.Factorial(n))
// 	fmt.Printf("Abs(%d) = %d\n", -42, mathutils.Abs(-42))
// 	fmt.Printf("IsEven(%d) = %v\n", 10, mathutils.IsEven(10))
// 	fmt.Printf("IsOdd(%d) = %v\n", 7, mathutils.IsOdd(7))
// }
