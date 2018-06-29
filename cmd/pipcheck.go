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

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/clearlinux/diva/internal/config"
	"github.com/clearlinux/diva/internal/helpers"
	"github.com/clearlinux/diva/internal/testutils"

	"github.com/spf13/cobra"
)

func init() {
	checkCmd.AddCommand(pipCmd)
	pipCmd.Flags().StringVarP(&pipFlags.path, "path", "p", "", "path to full chroot")
	pipCmd.Flags().UintVarP(&pipFlags.version, "version", "v", 0, "version to check")
}

type pipCmdFlags struct {
	version uint
	path    string
}

var pipFlags pipCmdFlags

var pipCmd = &cobra.Command{
	Use:   "pipcheck",
	Short: "Run pip check against full chroot",
	Long: `Run pip check against full chroot at <path> or constructed using
local configuration and <version>.`,
	Run: runPipCheck,
}

func runPipCheck(cmd *cobra.Command, args []string) {
	if pipFlags.version == 0 && pipFlags.path == "" {
		helpers.Fail(errors.New("must supply either --version or --path argument"))
	}

	p := pipFlags.path
	if p == "" {
		c, err := config.ReadConfig("")
		if err != nil {
			helpers.Fail(err)
		}
		p = filepath.Join(c.Mixer.MixWorkSpace, "update/image", fmt.Sprint(pipFlags.version), "full")
	}

	results := PipCheck(p)

	err := results.Print(os.Stdout)
	if err != nil {
		helpers.Fail(err)
	}

	if results.Failed > 0 {
		os.Exit(1)
	}
}

// PipCheck runs 'pip check' in a chroot at path
func PipCheck(path string) *testutils.Results {
	r := &testutils.Results{Name: "pipcheck"}
	name := "pipcheck"
	desc := "run pip check in full build root to check for missing python requirements"
	err := helpers.RunCommandSilent("chroot", path, "pip", "check")
	r.Add(name, desc, err, false)
	return r
}
