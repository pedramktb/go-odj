package main

import (
	"fmt"

	"github.com/pedramktb/go-odj"
)

var ExternalDependentVar = "External: " + odj.Version

func main() {
	fmt.Printf("Var: %s\n", ExternalDependentVar)
}
