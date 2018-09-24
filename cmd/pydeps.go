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

	"github.com/clearlinux/diva/bundle"
	"github.com/clearlinux/diva/diva"
	"github.com/clearlinux/diva/internal/helpers"
	"github.com/clearlinux/diva/pkginfo"

	"github.com/spf13/cobra"
)

type pyDepsCmdFlags struct {
	path      string
	version   string
	repoName  string
	buildroot bool
	bundleURL string
}

var pipFlags pyDepsCmdFlags

func init() {
	pyDepsCmd.Flags().StringVarP(&pipFlags.path, "path", "p", "", "path to full chroot")
	pyDepsCmd.Flags().StringVarP(&pipFlags.version, "version", "v", "0", "version to check")
	pyDepsCmd.Flags().StringVarP(&pipFlags.repoName, "reponame", "n", "clear", "Name of repo")
	pyDepsCmd.Flags().BoolVar(&pipFlags.buildroot, "buildroot", false, "construct build root from repo")
	pyDepsCmd.Flags().StringVarP(&pipFlags.bundleURL, "bundleurl", "b", "", "Upstream bundles url")

}

var pyDepsCmd = &cobra.Command{
	Use:   "pydeps",
	Short: "Run pip check against full chroot",
	Long: `Run pip check against full chroot at <path>, if a <path> is not specified
OR the --buildroot option is passed, a build root will be constructed using a
repo specified by <version> and <reponame>, which default to "0" and "clear",
respectively. If no <path> is passed, but the --buildroot option is, the build
root will be constructed here: "<conf.Mixer.MixWorkSpace>/update/image/<version>/full".`,
	Run: runCheckPyDeps,
}

func runCheckPyDeps(cmd *cobra.Command, args []string) {
	p := pipFlags.path
	if p == "" {
		p = filepath.Join(conf.Mixer.MixWorkSpace, "update/image", pipFlags.version, "full")
	}

	err := checkSystemRequirements()
	helpers.FailIfErr(err)

	if pipFlags.path == "" || pipFlags.buildroot {
		err := createFullChroot(p)
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

func createFullChroot(path string) error {
	var err error

	repo := pkginfo.Repo{
		URI:     "",
		Name:    pipFlags.repoName,
		Version: pipFlags.version,
		Type:    "B",
	}

	// populate the repo information from the database
	helpers.PrintBegin("Populating repo")
	err = pkginfo.PopulateRepo(&repo, conf.Paths.CacheLocation)
	if err != nil {
		return err
	}
	helpers.PrintComplete("Repo populated successfully")

	// create repo information
	err = helpers.RunCommandSilent("createrepo_c", repo.CacheDir)
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

	_, err = f.WriteString("[diva]\nname=diva\nbaseurl=file://" + repo.CacheDir)
	if err != nil {
		return err
	}

	if pipFlags.bundleURL != "" {
		conf.BundleDefsURL = pipFlags.bundleURL
	}

	// Get the latest bundle definitions if no version passed, otherwise checkout
	// the version tag
	if pipFlags.version == "0" {
		err = diva.GetLatestBundles(conf, "")
	} else {
		err = diva.GetBundlesAtTag(conf, pipFlags.version)
	}
	if err != nil {
		return err
	}

	// get the slice of all packages from all bundles for chroot install
	packages, err := bundle.GetAllPackagesForAllBundles(conf.Paths.BundleDefsRepo)
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
