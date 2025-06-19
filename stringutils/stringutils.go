// Package stringutils provides utility functions for string manipulation.
package stringutils

import (
	"strings"
	"unicode"
)

// Reverse returns the reverse of the input string.
func Reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// IsPalindrome checks if the given string is a palindrome (case-insensitive).
func IsPalindrome(s string) bool {
	s = strings.ToLower(s)
	return s == Reverse(s)
}

// Capitalize returns a string with the first letter capitalized.
func Capitalize(s string) string {
	if s == "" {
		return s
	}

	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// TrimSpaces removes all spaces from a string.
func TrimSpaces(s string) string {
	return strings.ReplaceAll(s, " ", "")
}

// WordCount returns the number of words in a string.
func WordCount(s string) int {
	return len(strings.Fields(s))
}
