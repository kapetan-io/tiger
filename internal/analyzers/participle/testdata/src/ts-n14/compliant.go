// The TS-N14 compliant rewrites: exported identifiers use nouns instead of
// participles, unexported identifiers stay silent regardless of shape, and
// the genuine-noun allowlist stays silent even though it ends in "-ing".
package fixture

// Preparation replaces the participle Preparing.
type Preparation struct{}

// StartProcess replaces StartProcessing.
func StartProcess() {
}

// Retry replaces Retrying.
const Retry = true

// Load replaces Loading.
var Load bool

// Order replaces the Job type and Pending field with noun-based names.
type Order struct {
	Status bool
}

// Sender replaces Reader/Closing.
type Sender struct{}

// Close replaces the participle method name Closing.
func (Sender) Close() {
}

// startProcessing is unexported, so it stays silent even though it ends in
// "-ing" — TS-N14 only covers exported identifiers.
func startProcessing() {
}

// Allowlisted is an exported type wrapping every default noun-allowlist
// entry as a field, proving each one stays silent.
type Allowlisted struct {
	String   string
	Encoding string
	Logging  bool
	Padding  int
	Warning  string
	Ring     int
	Thing    int
	Spring   int
	Sibling  int
	Setting  int
	King     int
	Wing     int
	Swing    int
}

// Finding is a genuine noun — what an analyzer reports — allowlisted after
// the tiger repository's own dogfood run flagged it (constraint 7).
type Finding struct{}

// SeverityBlocking carries the specification's own severity vocabulary;
// "blocking" is allowlisted for the same reason.
const SeverityBlocking = 0

// RoleBinding is a genuine noun — an association record — allowlisted after
// the querator trial flagged it (constraint 7).
type RoleBinding struct{}

// ProduceWaiting counts requests currently waiting; "waiting" is
// allowlisted for the same reason.
const ProduceWaiting = 0

// IngestsNothing ends in the indefinite pronoun "nothing" — "no" + "thing",
// not a participle of any verb — allowlisted after the git-server trial
// flagged TestEmptyPackIngestsNothing (constraint 7). The compounds are
// listed individually because the allowlist matches exact tokens: "thing"
// alone does not cover them.
const IngestsNothing = 0

// ReturnsSomething, MatchesAnything, and CoversEverything prove the rest of
// the indefinite-pronoun family stays silent for the same reason.
const (
	ReturnsSomething = 0
	MatchesAnything  = 0
	CoversEverything = 0
)
