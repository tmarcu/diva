// Copyright © 2018 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	defaultConfig = "/usr/share/defaults/diva/config.toml"
	systemConfig  = "/etc/diva/config.toml"
	userConfig    = ".config/diva/config.toml" // under $HOME
)

// mixConfig defines the path and configuration of the mix workspace used by diva
type mixConfig struct {
	MixWorkSpace string `required:"true"`
}

// pathConfig defines paths to various data used by diva
type pathConfig struct {
	BundleDefsRepo string
	LocalRPMRepo   string
	CacheLocation  string
}

// Config struct that defines the layout of the configuration file
type Config struct {
	Mixer mixConfig
	Paths pathConfig
}

func defaultConf() Config {
	ws := filepath.Join(os.Getenv("HOME"), "clearlinux")
	return Config{
		mixConfig{
			filepath.Join(ws, "mix"),
		},
		pathConfig{
			filepath.Join(ws, "projects/clr-bundles"),
			filepath.Join(ws, "repo"),
			filepath.Join(ws, "data"),
		},
	}
}

// ReadConfig reads configuration files on the system from default locations or
// at the path passed to configPath. The first configuration file found will be
// read. The configuration file paths are checked in the following order:
//
// defaultConfig "/usr/share/defaults/diva/config.toml"
// systemConfig  "/etc/diva/config.toml"
// userConfig    "$HOME/.config/diva/config.toml"
func ReadConfig(configPath string) (*Config, error) {
	var err error
	c := defaultConf()
	userConfPath := filepath.Join(os.Getenv("HOME"), userConfig)
	// return the first configuration file found, check in the following order
	order := []string{configPath, userConfPath, systemConfig, defaultConfig}
	for _, path := range order {
		// configPath may be empty
		if path == "" {
			continue
		}
		_, err = toml.DecodeFile(path, &c)
		// if the file isn't found, try the next one
		if os.IsNotExist(err) {
			continue
		}
		// file found, return result of decode
		return &c, err
	}

	// no configuration file, return compiled defaults
	return &c, nil
}
