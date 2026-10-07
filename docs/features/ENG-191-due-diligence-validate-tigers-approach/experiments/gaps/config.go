package gaps

// SpinConfig holds settings read from a config file at startup.
type SpinConfig struct {
	Spins int
}

// spinsMax is the most spins any config may ask for.
const spinsMax = 10_000

// SpinFromConfigRaw passes a config value straight to the loop's limit, so the
// file decides how long the loop may run.
func SpinFromConfigRaw(config SpinConfig, done func() bool) error {
	return SpinReported(done, config.Spins)
}

// SpinFromConfig clamps the config value to spinsMax where it enters the program.
func SpinFromConfig(config SpinConfig, done func() bool) error {
	return SpinReported(done, min(config.Spins, spinsMax))
}
