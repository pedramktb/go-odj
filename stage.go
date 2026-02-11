package odj

import (
	"github.com/pedramktb/go-envy"
)

type DeploymentStage string

const (
	StageTest  DeploymentStage = "test"
	StageDev   DeploymentStage = "dev"
	StageQA    DeploymentStage = "qa"
	StageProd  DeploymentStage = "prod"
	StageLocal DeploymentStage = ""
)

func (s DeploymentStage) String() string {
	if s == "" {
		return "local"
	}
	return string(s)
}

var Stage = func() DeploymentStage {
	stage, _, _ := envy.Get[string]("ODJ_EE_STAGE")
	return DeploymentStage(stage)
}()

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
