package odj

import (
	"context"
	"net/http"

	"github.com/go-faster/jx"
	"github.com/pedramktb/go-envy"
)

// GitSHA is the Git commit SHA of the binary, which is expected to be set at build time using ldflags.
// Defaults to an empty string if not set.
// e.g. -ldflags="-X github.com/pedramktb/go-odj.GitSHA=$(git rev-parse HEAD)"
var GitSHA string

// BuildDate is the build date of the binary, which is expected to be set at build time using ldflags.
// Defaults to an empty string if not set.
// e.g. -ldflags="-X github.com/pedramktb/go-odj.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var BuildDate string

// Version is the version core part (vM.m.p) of the semver of the binary, which is expected to be set at build time using ldflags.
// Defaults to "dev" if not set.
// e.g. -ldflags="-X github.com/pedramktb/go-odj.Version=1.2.3"
var Version string

// Iter is the build number in the Azure DevOps pipeline, which is expected to be set at build time using ldflags.
// Defaults to an empty string if not set.
// e.g. -ldflags="-X github.com/pedramktb/go-odj.Iter=$(Build.BuildNumber)"
var Iter string

// FullVersion is the full semver string of the binary, which is constructed based on the Version, Iter, and Stage variables.
// It follows the format "version[-preRelease.iter]" where preRelease is determined by the Stage:
// - For StageProd and StageTest, there is no preRelease suffix.
// - For StageQA, the preRelease suffix is "-rc".
// - For StageDev, the preRelease suffix is "-beta".
// - For any other stage (including StageLocal), the preRelease suffix is "-alpha" if the version is not "dev".
// If Iter is empty, it will be omitted from the version string.
var FullVersion = func() string {
	version := Version
	if version == "" {
		Version = "dev"
		version = "dev"
	}
	if Iter == "" {
		return version
	}
	if version == "dev" {
		return version + "." + Iter
	}
	switch Stage {
	case StageProd, StageTest:
		return version
	case StageQA:
		return version + "-rc." + Iter
	case StageDev:
		return version + "-beta." + Iter
	default:
		return version + "-alpha." + Iter
	}
}()

// Product is the product name of the ODJ component, determined by the ODJ_EE_PRODUCT environment variable.
// It defaults to "unknown" if not set or empty.
var Product = func() string {
	product, _, _ := envy.Get[string]("ODJ_EE_PRODUCT")
	if product != "" {
		return product
	}
	return "unknown"
}()

// Component is the component name of the ODJ component, determined by the ODJ_EE_COMPONENT environment variable.
// It defaults to "unknown" if not set or empty.
var Component = func() string {
	comp, _, _ := envy.Get[string]("ODJ_EE_COMPONENT")
	if comp != "" {
		return comp
	}
	return "unknown"
}()

// InfoHandler returns an HTTP handler function that serves build and version information as a JSON response.
func InfoHandler(deps ...func(ctx context.Context) (depName string, jsonBytes []byte)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		e := jx.GetEncoder()
		defer jx.PutEncoder(e)
		e.ObjStart()

		e.FieldStart("build_date")
		e.Str(BuildDate)

		e.FieldStart("version")
		e.Str(FullVersion)

		e.FieldStart("product")
		e.Str(Product)

		e.FieldStart("component")
		e.Str(Component)

		e.FieldStart("stage")
		e.Str(Stage.String())

		e.ObjEnd()

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if _, err := e.WriteTo(w); err != nil {
			return
		}
	}
}
