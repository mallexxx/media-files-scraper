package main

func mapSlice[T any, M any](a []T, f func(T) M) []M {
	n := make([]M, len(a))
	for i, e := range a {
		n[i] = f(e)
	}
	return n
}

func filterSlice[T any](a []T, f func(T) bool) []T {
	var n []T
	for _, e := range a {
		if f(e) {
			n = append(n, e)
		}
	}
	return n
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func findIndex[T comparable](arr []T, target T) int {
	for i, item := range arr {
		if item == target {
			return i
		}
	}
	return -1
}
