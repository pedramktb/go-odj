package odj

import (
	"os"
)

// Ensure ReloadEnv is called during variable initialization.
// This allows other package-level variables to depend on the populated values
func init() {
	ReloadEnv()
}

// ReloadEnv reloads the environment by reloading all environment variables and derived values.
// It updates:
//   - Stage
//   - SIAMMembershipStage
//   - Product
//   - Component
//   - FullVersion
func ReloadEnv() {
	// Stage logic
	stage := os.Getenv("ODJ_EE_STAGE")
	Stage = DeploymentStage(stage)

	switch Stage {
	case StageProd:
		SIAMMembershipStage = "prod"
	case StageTest,
		StageDev,
		StageQA,
		StageLocal:
		fallthrough
	default:
		SIAMMembershipStage = "test"
	}

	// Product logic
	product := os.Getenv("ODJ_EE_PRODUCT")
	if product != "" {
		Product = product
	} else {
		Product = "unknown"
	}

	// Component logic
	comp := os.Getenv("ODJ_EE_COMPONENT")
	if comp != "" {
		Component = comp
	} else {
		Component = "unknown"
	}

	// FullVersion logic
	version := Version
	if version == "" {
		Version = "dev"
		version = "dev"
	}
	if Iter == "" {
		FullVersion = version
		return
	}
	if version == "dev" {
		FullVersion = version + "." + Iter
		return
	}
	switch Stage {
	case StageProd, StageTest:
		FullVersion = version
	case StageQA:
		FullVersion = version + "-rc." + Iter
	case StageDev:
		FullVersion = version + "-beta." + Iter
	default:
		FullVersion = version + "-alpha." + Iter
	}
}
