package loopholes


type SpinConfig struct {
	Spins int
}


const spinsMax = 10_000



func SpinFromConfigRaw(config SpinConfig, done func() bool) error {
	return SpinReported(done, config.Spins)
}


func SpinFromConfig(config SpinConfig, done func() bool) error {
	return SpinReported(done, min(config.Spins, spinsMax))
}
