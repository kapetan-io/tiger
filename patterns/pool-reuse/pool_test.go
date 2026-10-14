package poolreuse_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	poolreuse "github.com/kapetan-io/tiger/patterns/pool-reuse"
)

// TestUseWithResetLeavesNothingBehind fills a pooled buffer and reads back what
// was returned to the pool.
//
// Goal: after UseWithReset no field of the last user remains, so the next Get
// cannot see it.
func TestUseWithResetLeavesNothingBehind(t *testing.T) {
	for _, test := range []struct {
		name string
		user string
		data []byte
	}{
		{name: "user and data", user: "alice", data: []byte("secret")},
		{name: "user only", user: "bob"},
	} {
		t.Run(test.name, func(t *testing.T) {
			put := poolreuse.UseWithReset(test.user, test.data)
			require.NotNil(t, put)
			assert.Empty(t, put.User)
			assert.Empty(t, put.Data)
		})
	}
}

// TestUseWithEmptyResetLeaksLastUser fills a pooled buffer through the broken
// shape and reads back what was returned to the pool.
//
// Goal: the broken shape leaves the last user and data in the buffer it put
// back, which tiger accepts because a Reset runs before Put.
func TestUseWithEmptyResetLeaksLastUser(t *testing.T) {
	for _, test := range []struct {
		name string
		user string
		data []byte
	}{
		{name: "user and data", user: "alice", data: []byte("secret")},
	} {
		t.Run(test.name, func(t *testing.T) {
			put := poolreuse.UseWithEmptyReset(test.user, test.data)
			require.NotNil(t, put)
			assert.Equal(t, test.user, put.User)
			assert.Equal(t, test.data, put.Data)
		})
	}
}
