package parser

import (
	"encr.dev/internal/experiment"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

type typeParameterLookup map[string]*schema.TypeParameterRef

func (p *parser) IsEnabled(releaseStage experiment.Experiment) bool {
	return releaseStage.Enabled(p.cfg.Experiments)
}
