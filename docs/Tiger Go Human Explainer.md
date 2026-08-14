
Tiger Go is a restricted subset of go where the specification lives in the source and a machine checks that the code matches it.

Tiger Go is not a style guide. It is a restriction of Go chosen so that questions a reviewer normally answers by reading become questions a tool answers by checking. The formatting rules are a side effect, not the point.

## What it is
- **A restricted subset of Go, chosen for decidability.** Every restriction exists to make some analysis possible. No package-level mutable state, no `unsafe`, no reflection, no dynamic dispatch in verified code — each one shrinks the space a tool has to reason about, and together they make alias analysis, frame checking, and termination proofs tractable on real code. The restrictions compound: no single one is worth much, and completing a chain of them turns an intractable analysis into a cheap one.
- **A declaration layer carried alongside the code.** Invariants get names and live in one file. Limits carry the derivation that produced them. Functions declare their effects, the memory they may write, their preconditions, and the ranking expression that proves their loops terminate. Interfaces declare the faults they can express. These declarations are the specification, and they are written in the source rather than in a document that drifts.
- **An architecture where every nondeterministic effect is injectable.** Time, randomness, IO, scheduling, and identity all arrive through surfaces that can be swapped for deterministic implementations. This is not a testing convenience. It is what makes the machine-checked properties describe the running system rather than a model of it.
- **A conformance obligation on every surface.** The simulated implementation and the production one pass the same suite, and the simulated one is the more adversarial of the two. A fault the interface cannot express is a bug class the simulator is structurally blind to, so interfaces get designed around their failure modes rather than their happy paths.

## What it buys
- **Determinism as a dial rather than a binary.** One test suite runs fully simulated, partially simulated, or fully real, and the difference is configuration. A failure hands you a seed instead of a story, and a bug found once stays found.
- **Detection moved as far up the ladder as the language allows.** Compile error beats assertion, assertion beats test, test beats production. Named quantity types catch a swapped index at compile time. Declared effects catch a `time.Now()` three call frames down, which a package-level import ban never sees.
- **Whole-program properties instead of local ones.** An effect is transitive by construction, so it cannot be laundered through an indirection. Declaring that a function does no IO is a claim about everything beneath it, checked at every call edge.
- **Refactors that need no review.** With exact call graphs, declared effects, declared frames, and unique identifiers, a tool can prove that an extraction or a rename preserves behavior. A proven transform is not worth a human's attention.
- **The same gate for generated and hand-written code.** Trust attaches to what an artifact provably satisfies rather than to who produced it. This matters more each month.

## How review divides

The split is not by topic and not by directory. Every concern in the system has two parts, and the parts get different treatment.
- **Machines check the correspondence.** Does the code satisfy the invariant it names, stay inside its declared frame, terminate, cover every state, match its declared effect set, route its nondeterminism through an injectable surface, respect the declared layering. All of it mechanical, all of it on every commit.
- **Humans and AI review the declarations.** Is `SequenceMonotonic` the invariant this protocol actually needs. Should this function be permitted to touch the disk at all. Is 512-byte sectors true of the hardware we deploy on. Can the `Storage` interface express a torn write, a misdirected write, an `fsync` that lies.
- **The declarations are a layer, not a region.** They cut across every file. There is no part of the tree that is "the reviewed part" and no boundary to erode, because the effect set of a function determines which checks apply to it and the effect set is computed rather than decreed.
- **The economics are the argument.** Declarations run two to five percent of lines and change an order of magnitude less often than the code around them. Review attention concentrates where being wrong is most expensive, which is the opposite of how it usually gets spent.
- **The policy that falls out.** A change touching no declaration and passing CI merges without a human reading it. A change touching a declaration goes to someone who knows the domain, and nothing else about it is discussed, because everything else has already been checked.

## What no amount of tooling fixes
- **A machine can prove the code satisfies the specification. Nothing can prove the specification matches the world.** The code contains no representation of the world, so the gap is structural rather than an engineering shortfall. Every judgment item above is an instance of it.
- **Conformance becomes the target the moment it becomes the merge criterion.** The defense is that a proxy must cost more to fake than to satisfy, plus one external oracle. Mutation score is that oracle, because it cannot be raised without the tests genuinely improving. Rising conformance against a flat mutation score is a fire.
- **False positives are how a dialect degrades back into a style guide.** A blocking rule wrong two percent of the time across a thousand merges is twenty forced suppressions, and suppressions are the erosion. Track the count as a first-class metric and treat each one as a bug report against the analyzer.
- **The cost curve is exponential in the tail.** Reaching eighty percent mechanization takes off-the-shelf linters and an afternoon. The last few percent takes an effect system, a points-to analysis, an abstract interpreter, and a refactoring prover — a team, not a quarter. Worth it for a database, unnecessary for most software, and the judgment about which one you are building is itself irreducible.

