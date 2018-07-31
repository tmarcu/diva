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
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/clearlinux/diva/bundle"
	"github.com/clearlinux/diva/diva"
	"github.com/clearlinux/diva/internal/helpers"
	"github.com/clearlinux/diva/pkginfo"
	"github.com/spf13/cobra"
)

type bundleCmdFlags struct {
	repoName string
	version  string
	bundle   string
}

// flags passed in as args
var bundleFlags bundleCmdFlags

func init() {
	checkCmd.AddCommand(verifyBundlesCmd)
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
definition header TITLE. For a <bundle> or the default of all bundles. An
optional <reponame> and <version> may be used to specify a repo the bundle
packages completeness will run against with "clear" and "0" as the defaults.`,
	Run: runVerifyBundle,
}

func runVerifyBundle(cmd *cobra.Command, args []string) {

	repo := pkginfo.Repo{
		URI:     "",
		Name:    bundleFlags.repoName,
		Version: bundleFlags.version,
		Type:    "B",
	}

	helpers.PrintBegin("Populating repo")
	err := pkginfo.PopulateRepo(&repo)
	helpers.FailIfErr(err)
	helpers.PrintComplete("Repo populated successfully")

	err = diva.GetLatestBundles(conf, "")
	helpers.FailIfErr(err)

	result := diva.NewSuite("bundle-verify", "validate bundle correctness")
	bundles, err := checkAndGetBundleDefinitions(result)
	helpers.FailIfErr(err)

	checkBundleHeaderTitleMatchesFile(bundles, result)

	err = checkBundleComplete(&repo, bundles, result)
	helpers.FailIfErr(err)

	err = checkIfPundleDeletesExist(result)
	helpers.FailIfErr(err)

	if result.Failed > 0 {
		os.Exit(1)
	}
}

func checkAndGetBundleDefinitions(result *diva.Results) (bundle.Set, error) {

	bundles := make(bundle.Set)
	var err error

	if bundleFlags.bundle == "" {
		bundles, err = bundle.GetAll(conf.Paths.BundleDefsRepo)
	} else {
		var singleBundle *bundle.Definition
		singleBundle, err = bundle.GetDefinition(bundleFlags.bundle, conf.Paths.BundleDefsRepo)
		if singleBundle != nil {
			bundles[singleBundle.Name] = singleBundle
		}
	}

	result.Ok(err == nil, "no include loops")
	return bundles, err
}

func checkBundleHeaderTitleMatchesFile(bundles bundle.Set, result *diva.Results) {
	var failures []string
	for filename, bundle := range bundles {
		if filename != bundle.Header.Title {
			failures = append(failures, bundle.Name)
		}
	}
	result.Ok(len(failures) == 0, "'TITLE' headers match bundle file names")
	if len(failures) > 0 {
		result.Diagnostic("mismatched headers:\n" + strings.Join(failures, "\n"))
	}
}

func checkBundleComplete(repo *pkginfo.Repo, bundles bundle.Set, result *diva.Results) error {
	var err error
	var rpm *pkginfo.RPM
	var failures []string

	for _, bundle := range bundles {
		for pkg := range bundle.DirectPackages {
			rpm, err = pkginfo.GetRPM(repo, pkg)
			if rpm == nil || err != nil {
				failures = append(failures, fmt.Sprintf("%s from bundle %s", pkg, bundle.Name))
			}
		}
	}
	result.Ok(len(failures) == 0, "all packages found in repo")
	if len(failures) > 0 {
		result.Diagnostic("missing packages:\n" + strings.Join(failures, "\n"))
	}
	return nil
}

func checkIfPundleDeletesExist(result *diva.Results) error {
	var deleted []string
	var err error

	output, err := helpers.RunCommandOutput(
		"git", "-C", conf.Paths.BundleDefsRepo, "diff", "latest..HEAD", "packages",
	)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		matches := regexp.MustCompile(`^-[^-]`).FindStringSubmatch(scanner.Text())
		if len(matches) > 0 {
			deleted = append(deleted, scanner.Text())
		}
	}
	result.Ok(len(deleted) == 0, "package bundles not deleted in release")
	if len(deleted) > 0 {
		result.Diagnostic("deleted package bundles:\n" + strings.Join(deleted, "\n"))
	}
	return nil
}
