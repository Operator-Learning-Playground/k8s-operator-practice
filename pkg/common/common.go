package common

import "os"

func GetWd() string {
	wd := os.Getenv("WORK_DIR")
	if wd == "" {
		wd, _ = os.Getwd()
	}
	return wd
}
