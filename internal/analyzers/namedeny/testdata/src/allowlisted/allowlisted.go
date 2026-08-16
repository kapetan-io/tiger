// Package fixture proves the -allow flag silences one token while leaving
// the rest of the deny dictionary enforced.
package fixture

// helperRoute is allowlisted via -allow=helper=legacy vendor API name.
func helperRoute() {
}

// queueItems is allowlisted via -allow=items=queue domain noun.
func queueItems() {
}

// queueItem is silenced by the same entry: the allow list folds singular
// and plural exactly as the deny dictionary does.
func queueItem() {
}

// dataStore is not allowlisted, so the deny dictionary still fires.
func dataStore() { // want `TS-N12: "dataStore" uses semantically empty token "data"`
}
