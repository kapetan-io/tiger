package cgo

/*
#include <stdlib.h>
static int one(void) { return 1; }
*/
import "C"

func One() int { return int(C.one()) }
