// The TS-L05 failure modes: a method before its type, a method before its
// constructor, and a constructor before its type.
package fixture

// Ping is declared before Server's type: a reader meets behavior before
// fields.
func (s *Server) Ping() string { // want `TS-L05: method Ping is declared before its type Server`
	return "pong " + s.Addr
}

type Server struct {
	Addr string
}

func NewServer(addr string) *Server {
	return &Server{Addr: addr}
}

type Cache struct {
	items map[string]int
}

// Get is declared before Cache's constructor: a reader meets a method
// before the type is ever constructed.
func (c *Cache) Get(key string) int { // want `TS-L05: method Get is declared before its constructor NewCache`
	return c.items[key]
}

func NewCache() *Cache {
	return &Cache{items: map[string]int{}}
}

// newLimiter is declared before Limiter's type: a reader meets
// construction before the fields being constructed.
func newLimiter(rate int) *Limiter { // want `TS-L05: constructor newLimiter is declared before its type Limiter`
	return &Limiter{rate: rate}
}

type Limiter struct {
	rate int
}

// Get is declared before the generic type Box: the receiver's type-parameter
// brackets do not hide which type it belongs to.
func (b *Box[T]) Get() T { // want `TS-L05: method Get is declared before its type Box`
	return b.value
}

type Box[T any] struct {
	value T
}

// Key is the same with two type parameters.
func (p *Pair[K, V]) Key() K { // want `TS-L05: method Key is declared before its type Pair`
	return p.key
}

type Pair[K comparable, V any] struct {
	key K
	val V
}
