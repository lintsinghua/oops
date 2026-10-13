//go:build !windows

package platform

func secureDir(string) error { return nil }
