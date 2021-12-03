package main

import "regexp"

var digitCheck = regexp.MustCompile(`^-?\d+$`)

func CheckNumericOnly(str string) bool {
	return digitCheck.MatchString(str)
}
