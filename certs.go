package pack

import "strings"

type CertConfig struct {
	Build []string
	Run   []string
}

var (
	buildPrefix = "build:"
	runPrefix   = "run:"
)

func NewCertConfig(paths []string) CertConfig {
	cfg := CertConfig{}

	for _, path := range paths {
		if strings.HasPrefix(path, buildPrefix) {
			cfg.Build = append(cfg.Build, strings.TrimLeft(path, buildPrefix))
		} else if strings.HasPrefix(path, runPrefix) {
			cfg.Run = append(cfg.Run, strings.TrimLeft(path, runPrefix))
		} else {
			cfg.Build = append(cfg.Build, path)
			cfg.Run = append(cfg.Run, path)
		}
	}

	return cfg

}
