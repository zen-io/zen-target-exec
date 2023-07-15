package exec

import (
	zen_targets "github.com/zen-io/zen-core/target"
)

var KnownTargets = zen_targets.TargetCreatorMap{
	"exec": ExecConfig{},
}
