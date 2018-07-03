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
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/clearlinux/diva/bundle"
	"github.com/clearlinux/diva/diva"
	"github.com/clearlinux/diva/internal/config"
	"github.com/clearlinux/diva/internal/helpers"
	"github.com/clearlinux/diva/pkginfo"
	"github.com/spf13/cobra"
)

type bundleCmdFlags struct {
	repoURL  string
	repoName string
	version  string
	bundle   string
}

// flags passed in as args
var bundleFlags bundleCmdFlags

// config object used by GetUpstreamRepoFiles and called functions
var c *config.Config

func init() {
	var err error
	c, err = config.ReadConfig("")
	if err != nil {
		helpers.Fail(err)
	}

	checkCmd.AddCommand(verifyBundlesCmd)
	verifyBundlesCmd.Flags().StringVarP(&bundleFlags.repoURL, "repourl", "u", "", "Url to repo")
	verifyBundlesCmd.Flags().StringVarP(&bundleFlags.repoName, "reponame", "n", "clear", "Name of repo")
	verifyBundlesCmd.Flags().StringVarP(&bundleFlags.version, "version", "v", "0", "Version to check")
	verifyBundlesCmd.Flags().StringVarP(&bundleFlags.bundle, "bundle", "b", "", "Bundle to check")
}

var verifyBundlesCmd = &cobra.Command{
	Use:   "bundles",
	Short: "Verify bundle definitions are complete, and packages exist within repo.",
	Long: `Verify bundles are complete by checking that all named packages within
bundle and package bundle files can be found in the configured repo. It also
ensures no include loops exist, and that the bundle filename matches the bundle
definition header title. A <path> to a directory of rpms must be passed, and
will run against all bundles or a <bundle>. A <version> and <reponame> can used
to correctly document data, and are defaulted to 0, and "clear", respectively.`,
	Run: runVerifyBundle,
}

func runVerifyBundle(cmd *cobra.Command, args []string) {
	if bundleFlags.repoURL == "" {
		helpers.Fail(errors.New("must supply a --repourl argument"))
	}

	repo := pkginfo.Repo{
		URI:     bundleFlags.repoURL,
		Name:    bundleFlags.repoName,
		Version: bundleFlags.version,
		Type:    "B",
	}

	if err := pkginfo.ImportAllRPMs(&repo); err != nil {
		helpers.Fail(err)
	}

	if err := diva.GetLatestBundles(c, ""); err != nil {
		helpers.Fail(err)
	}

	result := &diva.Results{Name: "bundle-verify"}
	bundles := checkAndGetBundleDefinitions(result)
	checkBundleHeaderTitleMatchesFile(bundles, result)
	checkBundleComplete(&repo, bundles, result)
	checkIfPundleDeletesExist(result)

	err := result.Print(os.Stdout)
	if err != nil {
		helpers.Fail(err)
	}

	if result.Failed > 0 {
		os.Exit(1)
	}
}

func checkAndGetBundleDefinitions(result *diva.Results) bundle.Set {
	name := "bundle definition creation"
	desc := "Create bundle definitions, while checking for correctness, and no include loops."

	bundles := make(bundle.Set)
	var err error

	if bundleFlags.bundle == "" {
		bundles, err = bundle.GetAll(c.Paths.BundleDefsRepo)
	} else {
		var singleBundle *bundle.Definition
		singleBundle, err = bundle.GetDefinition(bundleFlags.bundle, c.Paths.BundleDefsRepo)
		if singleBundle != nil {
			bundles[singleBundle.Name] = singleBundle
		}
	}

	result.Add(name, desc, err, false)
	return bundles
}

func checkBundleHeaderTitleMatchesFile(bundles bundle.Set, result *diva.Results) {
	name := "bundle filename and title matching"
	desc := "Ensure that the bundle header 'TITLE' and bundle file name are equal."

	for filename, bundle := range bundles {
		if filename != bundle.Header.Title {
			err := fmt.Errorf("Bundle filename '%s' does not match Header title '%s'", filename, bundle.Header.Title)
			result.Add(name, desc, err, false)
			return
		}
	}
	result.Add(name, desc, nil, false)
}

func checkBundleComplete(repo *pkginfo.Repo, bundles bundle.Set, result *diva.Results) {
	name := "bundle package repo existence"
	desc := "Verify named packages from bundles exist in a repo"

	var missing []string
	var err error
	var rpm *pkginfo.RPM

	for _, bundle := range bundles {
		for pkg := range bundle.DirectPackages {
			rpm, err = pkginfo.GetRPM(repo, pkg)
			if err != nil {
				result.Add(name, desc, err, false)
				return
			}
			if rpm == nil {
				missing = append(missing, fmt.Sprintf("%s from %s not found", pkg, bundle.Name))
			}
		}
	}
	if len(missing) > 0 {
		err = fmt.Errorf(strings.Join(missing, "\n"))
	}
	result.Add(name, desc, err, false)
}

func checkIfPundleDeletesExist(result *diva.Results) {
	name := "package bundle delete"
	desc := "Determine whether a package bundle was deleted"

	var deleted []string
	var err error

	output, err := helpers.RunCommandOutput(
		"git", "-C", c.Paths.BundleDefsRepo, "diff", "latest..HEAD", "packages",
	)
	if err != nil {
		result.Add(name, desc, err, false)
		return
	}

	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		if matches := regexp.MustCompile(`^-[^-]`).FindStringSubmatch(scanner.Text()); len(matches) > 0 {
			deleted = append(deleted, fmt.Sprintf("Pundle delete detected: %s", scanner.Text()))
		}
	}
	if len(deleted) > 0 {
		err = fmt.Errorf(strings.Join(deleted, "\n"))
	}
	result.Add(name, desc, err, false)
}
