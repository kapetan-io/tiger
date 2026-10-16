package web

import "example.com/fakedb"

var pool, _ = fakedb.Open("data")

func init() { _ = pool.Exec("start") }
