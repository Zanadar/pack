package extend

import (
	"io/ioutil"
	"path/filepath"
	"strings"
)

type CertConfig struct {
	Build Certs
	Run   Certs
}

const (
	buildPrefix = "build:"
	runPrefix   = "run:"
	sep         = ","
)

// Populates a CertConfig with the contents of certs at paths
// TODO validate these are '.crt' files
func NewCertConfig(paths []string) (CertConfig, error) {
	cfg := CertConfig{}

	if len(paths) == 1 {
		paths = strings.Split(paths[0], sep)

	}
	for _, path := range paths {
		fullPath, err := filepath.Abs(path)
		if err != nil {
			return CertConfig{}, err
		}
		contents, err := ioutil.ReadFile(fullPath)
		if err != nil {
			return CertConfig{}, err
		}

		contString := string(contents)
		if strings.HasPrefix(path, buildPrefix) {
			cfg.Build = append(cfg.Build, contString)
		} else if strings.HasPrefix(path, runPrefix) {
			cfg.Run = append(cfg.Run, contString)
		} else {
			cfg.Build = append(cfg.Build, contString)
			cfg.Run = append(cfg.Run, contString)
		}
	}

	return cfg, nil

}
