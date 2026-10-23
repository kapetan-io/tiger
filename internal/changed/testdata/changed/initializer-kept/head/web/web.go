package web

import "example.com/fakedb"

var pool, _ = fakedb.Open("data")

var _, _ = pool.Query("key")

func init() { _ = pool.Exec("start") }
