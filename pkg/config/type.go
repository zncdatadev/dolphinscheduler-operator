package config

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
)

var configLogger = ctrl.Log.WithName("config")

type Configuration interface {
	ComputeEnv() (map[string]string, error)
	ComputeFile() (map[string]map[string]string, error)
	ComputeCli() (map[string]string, error)
}

//type ConfigGenerator[T string | map[string]string] interface {
//	Generate() T
//}

// FileContentGenerator generate config
// we can use this interface to generate config content
// and use GenerateAllFile function to generate configMap data
type FileContentGenerator interface {
	Generate(ctx context.Context) (string, error)
	FileName() string
}

type EnvGenerator interface {
	Generate(ctx context.Context) (map[string]string, error)
}

func GenerateAllFile(ctx context.Context, confGenerator []FileContentGenerator) map[string]string {
	data := make(map[string]string)
	for _, generator := range confGenerator {
		content, err := generator.Generate(ctx)
		if err != nil {
			configLogger.Error(err, "generate config error", "generator", generator, "fileName", generator.FileName())
			continue
		}

		if content != "" {
			data[generator.FileName()] = content
		}
	}
	return data
}

func GenerateAllEnv(ctx context.Context, confGenerator []EnvGenerator) map[string]string {
	data := make(map[string]string)
	for _, generator := range confGenerator {
		content, err := generator.Generate(ctx)
		if err != nil {
			configLogger.Error(err, "generate env config error", "generator", generator)
		}
		for k, v := range content {
			data[k] = v
		}
	}
	return data
}
