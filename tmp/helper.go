package main

import (
	"fmt"
	"runtime"
)

func getOperatingSystem() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return "Linux", nil
	case "darwin":
		return "Darwin", nil
	case "windows":
		return "Windows", nil
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// func main() {
// 	os, err := getOperatingSystem()
// 	if err != nil {
// 		fmt.Println(err)
// 	} else {
// 		fmt.Println("Operating System:", os)
// 	}
// }
