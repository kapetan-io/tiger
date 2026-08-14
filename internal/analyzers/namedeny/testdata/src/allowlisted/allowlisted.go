// Package fixture proves the -allow flag silences one token while leaving
// the rest of the deny dictionary enforced.
package fixture

// helperRoute is allowlisted via -allow=helper=legacy vendor API name.
func helperRoute() {
}

// dataStore is not allowlisted, so the deny dictionary still fires.
func dataStore() { // want `TS-N12: "dataStore" uses semantically empty token "data"`
}
