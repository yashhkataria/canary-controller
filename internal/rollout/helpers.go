package rollout

import (
	"encoding/json"
	"fmt"
	"time"
)

func DefaultAnalysis() Analysis {
	return Analysis{
		Interval:      Duration(DefaultInterval),
		ChecksPerStep: DefaultChecksPerStep,
		FailureLimit:  DefaultFailureLimit,
		MaxErrorRate:  DefaultMaxErrorRate,
		ErrorMargin:   DefaultErrorMargin,
		MaxP99Ms:      DefaultMaxP99Ms,
		LatencyFactor: DefaultLatencyFactor,
		MinRequests:   DefaultMinRequests,
		MaxWait:       Duration(DefaultMaxWait),
	}
}

func (p Phase) Terminal() bool {
	return p == PhaseSucceeded || p == PhaseRolledBack
}

func (d Duration) Std() time.Duration {
	return time.Duration(d)
}

func (d Duration) String() string {
	return time.Duration(d).String()
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("duration must be a string like \"20s\"")
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q", s)
	}
	*d = Duration(v)
	return nil
}
