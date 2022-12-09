//$GOROOT/bin/go run $(dirname $0)/mage.go $@ ; exit
//go:build ignore
// +build ignore

package main

import (
	"os"

	"github.com/magefile/mage/mage"
)

func main() { os.Exit(mage.Main()) }
