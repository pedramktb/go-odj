package odj

import (
	"github.com/pedramktb/go-envy"
)

// DeploymentStage is the type that defines deployments stages in ODJ
type DeploymentStage string

const (
	StageTest  DeploymentStage = "test"
	StageDev   DeploymentStage = "dev"
	StageQA    DeploymentStage = "qa"
	StageProd  DeploymentStage = "prod"
	StageLocal DeploymentStage = ""
)

// String returns the string representation of the deployment stage, with "local" as the default for empty stages.
func (s DeploymentStage) String() string {
	if s == "" {
		return "local"
	}
	return string(s)
}

// Stage is the current deployment stage, determined by the ODJ_EE_STAGE environment variable. It defaults to "local" if not set or empty.
var Stage = func() DeploymentStage {
	stage, _, _ := envy.Get[string]("ODJ_EE_STAGE")
	return DeploymentStage(stage)
}()

// SIAMMembershipStage is the stage value used for SIAM membership, which maps "prod" to "prod" and all other stages to "test".
var SIAMMembershipStage = func() string {
	switch Stage {
	case StageProd:
		return "prod"
	case StageTest,
		StageDev,
		StageQA,
		StageLocal:
		fallthrough
	default:
		return "test"
	}
}()
