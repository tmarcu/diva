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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/clearlinux/diva/diva"
	"github.com/clearlinux/diva/internal/config"
	"github.com/clearlinux/diva/internal/helpers"
	"github.com/clearlinux/diva/pkginfo"

	"github.com/spf13/cobra"
)

type pyDepsCmdFlags struct {
	mixName   string
	version   string
	latest    bool
	path      string
	buildroot bool
}

var pipFlags pyDepsCmdFlags

func init() {
	pyDepsCmd.Flags().StringVarP(&pipFlags.mixName, "name", "n", "clear", "name of data group")
	pyDepsCmd.Flags().StringVarP(&pipFlags.version, "version", "v", "0", "version to check")
	pyDepsCmd.Flags().BoolVar(&pipFlags.latest, "latest", false, "get the latest version from upstreamURL")
	pyDepsCmd.Flags().StringVarP(&pipFlags.path, "path", "p", "", "path to full chroot")
	pyDepsCmd.Flags().BoolVar(&pipFlags.buildroot, "buildroot", false, "construct build root from repo")
}

var pyDepsCmd = &cobra.Command{
	Use:   "pydeps",
	Short: "Run pip check against full chroot",
	Long: `Run pip check against full chroot at <path>, if a <path> is not specified
OR the --buildroot option is passed, a build root will be constructed using a
repo specified by <version> and <name>, which default to "0" and "clear",
respectively. If no <path> is passed, but the --buildroot option is, the build
root will be constructed here: "<conf.Mixer.MixWorkSpace>/update/image/<version>/full".
NOTE: This command may need root privileges: 'sudo -E'.`,
	Run: runCheckPyDeps,
}

func runCheckPyDeps(cmd *cobra.Command, args []string) {
	p := pipFlags.path
	if p == "" {
		p = filepath.Join(conf.Mixer.MixWorkSpace, "update/image", pipFlags.version, "full")
	}

	u := config.UInfo{
		MixName: pipFlags.mixName,
		Ver:     pipFlags.version,
		Latest:  pipFlags.latest,
	}

	repo, err := pkginfo.NewRepo(conf, &u)
	helpers.FailIfErr(err)

	bundleInfo, err := pkginfo.NewBundleInfo(conf, &u)
	helpers.FailIfErr(err)

	err = checkSystemRequirements()
	helpers.FailIfErr(err)

	if pipFlags.path == "" || pipFlags.buildroot {
		err := createFullChroot(p, &bundleInfo, &repo)
		helpers.FailIfErr(err)
	}

	results := CheckPyDeps(p)
	if results.Failed > 0 {
		os.Exit(1)
	}
}

// Check both rpm and pip are installed on the system, to be used by the Pipcheck
// and that the local chroot exists, or a repoURL is provided.
func checkSystemRequirements() error {
	for _, tool := range []string{"createrepo_c", "dnf", "pip"} {
		err := helpers.RunCommandSilent(tool, "--version")
		if err != nil {
			return err
		}
	}
	return nil
}

func createFullChroot(path string, bundleInfo *pkginfo.BundleInfo, repo *pkginfo.Repo) error {
	var err error

	// populate the repo information from the database
	helpers.PrintBegin("Populating repo")
	err = pkginfo.PopulateRepo(repo)
	if err != nil {
		return err
	}
	helpers.PrintComplete("Repo populated successfully")

	// create repo information
	err = helpers.RunCommandSilent("createrepo_c", repo.RPMCache)
	if err != nil {
		return err
	}

	// create dnf config file with repo information
	tmpDir, err := ioutil.TempDir("", "dnfconfig")
	if err != nil {
		return err
	}

	dnfConf := filepath.Join(tmpDir, "dnf.conf")
	f, err := os.Create(dnfConf)
	if err != nil {
		return err
	}

	defer func() {
		_ = f.Close()
		_ = os.RemoveAll(tmpDir)
	}()

	_, err = f.WriteString("[diva]\nname=diva\nbaseurl=file://" + repo.RPMCache)
	if err != nil {
		return err
	}

	err = pkginfo.PopulateBundles(bundleInfo, "")
	if err != nil {
		return err
	}

	// get the slice of all packages from all bundles for chroot install
	pkgsmap, err := bundleInfo.BundleDefinitions.GetAllPackages("")
	if err != nil {
		return err
	}

	packages, err := helpers.HashmapToSortedSlice(pkgsmap)
	if err != nil {
		return err
	}

	// clean the cache and metadata first
	err = helpers.RunCommandSilent("dnf", "-c", dnfConf, "clean", "all")
	if err != nil {
		return err
	}

	dnfArgs := []string{"-c", dnfConf, "--installroot=" + path, "install", "-y",
		"--releasever=" + pipFlags.version}

	// install all bundle packages into full chroot, starting with filesystem
	helpers.PrintBegin("Preparing build root at %s", path)
	err = helpers.RunCommandSilent("dnf", append(dnfArgs, "filesystem")...)
	if err != nil {
		return err
	}

	err = helpers.RunCommandSilent("dnf", append(dnfArgs, packages...)...)
	if err != nil {
		return err
	}

	helpers.PrintComplete("Done preparing build root")
	return nil
}

// CheckPyDeps runs 'pip check' in a chroot at path
func CheckPyDeps(path string) *diva.Results {
	name := "Python dependencies"
	desc := "run pip check in full build root to check for missing python requirements"
	r := diva.NewSuite(name, desc)
	r.Header(1)

	err := helpers.RunCommandSilent("chroot", path, "pip", "check")
	r.Ok(err == nil, desc)
	if err != nil {
		r.Diagnostic(err.Error())
	}
	return r
}
