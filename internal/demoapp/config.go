package demoapp

func ConfigFromEnv(getenv func(string) string) (Config, error) {
	errorRate, err := parseErrorRate(getenv(envErrorRate))
	if err != nil {
		return Config{}, err
	}
	extraLatency, err := parseExtraLatency(getenv(envExtraLatency))
	if err != nil {
		return Config{}, err
	}
	return Config{
		Version:      withDefault(getenv(envVersion), DefaultVersion),
		Track:        withDefault(getenv(envTrack), DefaultTrack),
		ErrorRate:    errorRate,
		ExtraLatency: extraLatency,
	}, nil
}
