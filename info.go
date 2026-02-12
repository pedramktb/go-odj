package odj

import (
	"context"
	"net/http"

	"github.com/go-faster/jx"
	"github.com/pedramktb/go-envy"
)

var GitSHA, BuildDate, Version, Iter string

var FullVersion = func() string {
	version := Version
	if version == "" {
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

var Product = func() string {
	product, _, _ := envy.Get[string]("ODJ_EE_PRODUCT")
	if product != "" {
		return product
	}
	return "unknown"
}()

var Component = func() string {
	comp, _, _ := envy.Get[string]("ODJ_EE_COMPONENT")
	if comp != "" {
		return comp
	}
	return "unknown"
}()

func InfoHandler(deps ...func(ctx context.Context) (depName string, jsonBytes []byte)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		e := jx.GetEncoder()
		defer jx.PutEncoder(e)
		e.ObjStart()

		e.FieldStart("git_sha")
		e.Str(GitSHA)

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
