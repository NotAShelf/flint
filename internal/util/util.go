package util

import (
	"os"
)

func IsNoColor() bool {
	_, noColor := os.LookupEnv("NO_COLOR")
	return noColor
}
