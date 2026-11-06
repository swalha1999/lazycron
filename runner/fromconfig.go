package runner

import (
	"github.com/swalha1999/lazycron/config"
)

// NewExecutor builds an executor for this machine from ~/.lazycron/config.yml
// settings. herdr-core is looked up here; when it is missing the executor
// falls back to plain runs and records why.
func NewExecutor(paths Paths, cfg *config.Config) *Executor {
	e := &Executor{
		Paths:    paths,
		SpaceID:  cfg.Herdr.SpaceID,
		KeepRuns: cfg.Herdr.KeepRunsOrDefault(),
		Hold:     config.DefaultHold,

		ExtraLabels: cfg.Herdr.Labels,
	}
	if hold, err := cfg.Herdr.HoldDuration(); err == nil {
		e.Hold = hold
	}
	if bin, err := FindHC(cfg.Herdr.Bin); err != nil {
		e.HCUnavailable = err.Error()
	} else {
		e.HC = &HC{Bin: bin, Cmd: LocalCommander{}}
	}
	if e.SpaceID == "" && cfg.EnsureSpaceID() {
		e.SpaceID = cfg.Herdr.SpaceID
		_ = config.Save(cfg) // best effort: keep the space stable across runs
	}
	e.NameSpace = func() { NameAppSpace(e.SpaceID, config.DefaultSpaceName) }
	return e
}
