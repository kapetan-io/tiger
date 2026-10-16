// Re-exporting internals for tests, the export_test.go habit, gets no exception.
package fixture // want `TS-T12: this test file declares package fixture`

var Hidden = hidden
