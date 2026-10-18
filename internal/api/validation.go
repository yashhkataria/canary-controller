package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/yashhkataria/canary-controller/internal/rollout"
	"github.com/yashhkataria/canary-controller/internal/store"
)

func validateCreate(req CreateRolloutRequest) map[string]string {
	fields := map[string]string{}
	checkName(fields, "namespace", req.Namespace)
	checkName(fields, "app", req.App)
	checkImage(fields, req.Image)
	checkSteps(fields, req.Steps)
	checkAnalysis(fields, req.Analysis.merged())
	return fields
}

func checkName(fields map[string]string, key, v string) {
	switch {
	case v == "":
		fields[key] = "is required"
	case len(v) > maxNameLength:
		fields[key] = fmt.Sprintf("must be at most %d characters", maxNameLength)
	case !namePattern.MatchString(v):
		fields[key] = "must be lowercase letters, digits and '-', starting and ending with a letter or digit"
	}
}

func checkImage(fields map[string]string, v string) {
	switch {
	case v == "":
		fields["image"] = "is required"
	case len(v) > maxImageLength:
		fields["image"] = fmt.Sprintf("must be at most %d characters", maxImageLength)
	case strings.IndexFunc(v, unicode.IsSpace) >= 0:
		fields["image"] = "must not contain spaces"
	}
}

func checkSteps(fields map[string]string, steps []int) {
	if len(steps) == 0 {
		fields["steps"] = "is required"
		return
	}
	if len(steps) > maxSteps {
		fields["steps"] = fmt.Sprintf("must have at most %d entries", maxSteps)
		return
	}
	prev := 0
	for i, w := range steps {
		switch {
		case w < stepUnit || w > 100:
			fields["steps"] = fmt.Sprintf("steps[%d] = %d: each step must be between %d and 100", i, w, stepUnit)
		case w%stepUnit != 0:
			fields["steps"] = fmt.Sprintf("steps[%d] = %d: each step must be a multiple of %d (%d replicas)", i, w, stepUnit, rollout.TotalReplicas)
		case w <= prev:
			fields["steps"] = fmt.Sprintf("steps[%d] = %d: steps must be strictly increasing", i, w)
		}
		if fields["steps"] != "" {
			return
		}
		prev = w
	}
}

func checkAnalysis(fields map[string]string, a rollout.Analysis) {
	if a.Interval.Std() < minInterval {
		fields["analysis.interval"] = fmt.Sprintf("must be at least %s", minInterval)
	}
	if a.ChecksPerStep < 1 {
		fields["analysis.checksPerStep"] = "must be at least 1"
	}
	if a.FailureLimit < 1 {
		fields["analysis.failureLimit"] = "must be at least 1"
	}
	if a.MaxErrorRate < 0 || a.MaxErrorRate > 1 {
		fields["analysis.maxErrorRate"] = "must be between 0 and 1"
	}
	if a.ErrorMargin < 0 || a.ErrorMargin > 1 {
		fields["analysis.errorMargin"] = "must be between 0 and 1"
	}
	if a.MaxP99Ms <= 0 {
		fields["analysis.maxP99Ms"] = "must be greater than 0"
	}
	if a.LatencyFactor < 1 {
		fields["analysis.latencyFactor"] = "must be at least 1"
	}
	if a.MinRequests < 0 {
		fields["analysis.minRequests"] = "must be at least 0"
	}
	if a.MaxWait.Std() < a.Interval.Std() {
		fields["analysis.maxWait"] = "must be at least the interval"
	}
}

func (a AnalysisRequest) merged() rollout.Analysis {
	out := rollout.DefaultAnalysis()
	if a.Interval != nil {
		out.Interval = *a.Interval
	}
	if a.ChecksPerStep != nil {
		out.ChecksPerStep = *a.ChecksPerStep
	}
	if a.FailureLimit != nil {
		out.FailureLimit = *a.FailureLimit
	}
	if a.MaxErrorRate != nil {
		out.MaxErrorRate = *a.MaxErrorRate
	}
	if a.ErrorMargin != nil {
		out.ErrorMargin = *a.ErrorMargin
	}
	if a.MaxP99Ms != nil {
		out.MaxP99Ms = *a.MaxP99Ms
	}
	if a.LatencyFactor != nil {
		out.LatencyFactor = *a.LatencyFactor
	}
	if a.MinRequests != nil {
		out.MinRequests = *a.MinRequests
	}
	if a.MaxWait != nil {
		out.MaxWait = *a.MaxWait
	}
	return out
}

func parseListFilter(r *http.Request) (store.ListFilter, map[string]string) {
	q := r.URL.Query()
	f := store.ListFilter{Namespace: q.Get("namespace"), App: q.Get("app"), Limit: store.DefaultListLimit}
	fields := map[string]string{}

	checkName(fields, "app", f.App)
	if f.Namespace != "" {
		checkName(fields, "namespace", f.Namespace)
	}
	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > store.MaxListLimit {
			fields["limit"] = fmt.Sprintf("must be a whole number from 1 to %d", store.MaxListLimit)
		} else {
			f.Limit = n
		}
	}
	return f, fields
}
