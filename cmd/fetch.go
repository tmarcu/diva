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
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/clearlinux/diva/internal/helpers"
	"github.com/clearlinux/diva/pkginfo"

	"github.com/spf13/cobra"
)

type allFetchFlags struct {
	version     uint
	bundleURL   string
	upstreamURL string
}

var allFlags allFetchFlags

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch and cache various build information",
}

var fetchAllCmd = &cobra.Command{
	Use:   "all [--version <version>] [--bundleurl|--upstreamurl <url>]",
	Run:   runFetchAllCmd,
	Short: "Fetch metadata from <version> or latest available if <version> is not supplied",
	Long: `Fetch latest bundle definitions, RPM repository, and update manifests from <version>.
If <version> is not supplied, fetch latest available content. Pass
--upstreamurl to fetch RPMs and update metadata from a location other than the
configured Upstream URL and --bundleurl to fetch the bundle definitions from a
location other than default.

RPMs will be cached under the cache location defined in your configuration or
default to $HOME/clearlinux/data/rpms/<version>. Bundle definition files will
be placed at $HOME/clearlinux/projects/clr-bundles by default. Update metadata
will be cached under the cache location as well, defaulting to
$HOME/clearlinux/data/update/<version>.`,
}

var fetchBundlesCmd = &cobra.Command{
	Use:   "bundles [--bundleurl <url>]",
	Run:   runFetchBundlesCmd,
	Short: "Fetch bundle definition files",
	Long: `Fetch bundle definition files from https://github.com/clearlinux/clr-bundles or
<url> if --bundleurl is supplied. Places the bundle definition files in
$HOME/clearlinux/projects/clr-bundles by default.`,
}

var fetchRepoCmd = &cobra.Command{
	Use:   "repo [--version <version>] [--upstreamurl <url>]",
	Run:   runFetchRepoCmd,
	Short: "Fetch repo from <version> or latest if not supplied",
	Long: `Fetch the RPM repository at <version> from the upstream URL if <version> is
supplied, otherwise fetch the latest available. If --upstreamurl is supplied, fetch
from <url> instead of the configured/default upstream URL. The repository is
cached under the cache location defined in your configuration or default to
$HOME/clearlinux/data/rpms/<version>.`,
}

var fetchUpdateCmd = &cobra.Command{
	Use:   "update [--version <version>] [--upstreamurl <url>]",
	Run:   runFetchUpdateCmd,
	Short: "Fetch the update at <version> or latest if not supplied",
	Long: `Fetch the update at <version> from the upstream URL if <version> is supplied,
otherwise fetch the latest available. If --upstreamurl is supplied, fetch from
<url> instead of the configured/default upstream URL. The update data is cached
under the cache location defined in your configuration or default to
$HOME/clearlinux/data/update/<version>.`,
}

var fetchCmds = []*cobra.Command{
	fetchAllCmd,
	fetchBundlesCmd,
	fetchRepoCmd,
	fetchUpdateCmd,
}

func init() {
	for _, cmd := range fetchCmds {
		fetchCmd.AddCommand(cmd)
	}

	rootCmd.AddCommand(fetchCmd)

	fetchAllCmd.Flags().UintVarP(&allFlags.version, "version", "v", 0, "version from which to pull data")
	fetchAllCmd.Flags().StringVarP(&allFlags.bundleURL, "bundleurl", "b", "", "URL from which to pull bundle definitions")
	fetchAllCmd.Flags().StringVarP(&allFlags.upstreamURL, "upstreamurl", "u", "", "URL from which to pull update metadata")

	fetchBundlesCmd.Flags().StringVarP(&allFlags.bundleURL, "bundleurl", "b", "", "URL from which to pull bundle definitions")

	fetchRepoCmd.Flags().UintVarP(&allFlags.version, "version", "v", 0, "version from which to pull data")

	fetchUpdateCmd.Flags().UintVarP(&allFlags.version, "version", "v", 0, "version from which to pull data")
	fetchUpdateCmd.Flags().StringVarP(&allFlags.upstreamURL, "upstreamurl", "u", "", "URL from which to pull update metadata")
}

type uinfo struct {
	ver uint
	url string
}

func getUpstreamInfo(allFlags allFetchFlags) (uinfo, error) {
	u := uinfo{}
	if allFlags.upstreamURL == "" {
		u.url = conf.UpstreamURL
	} else {
		u.url = allFlags.upstreamURL
	}

	if allFlags.version != 0 {
		u.ver = allFlags.version
		// no need to continue
		return u, nil
	}

	resp, err := http.Get(u.url + "/latest")
	if err != nil {
		return u, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return u, err
	}

	verString := strings.Trim(string(body), "\n")
	v, err := strconv.ParseUint(verString, 10, 32)
	u.ver = uint(v)
	return u, err
}

func fetchRepo(u uinfo) error {
	repo := &pkginfo.Repo{
		URI:     fmt.Sprintf("%s/releases/%d/clear/x86_64/os/", u.url, u.ver),
		Name:    "clear",
		Version: u.ver,
		Type:    "B",
	}

	helpers.PrintBegin("fetching repo from %s", repo.URI)
	path, err := pkginfo.GetRepoFiles(repo)
	if err != nil {
		return err
	}
	helpers.PrintComplete("repo cached at %s", path)
	return nil
}

func getLatestBundles(url string) error {
	if url == "" {
		url = conf.BundleDefsURL
	}

	if _, err := os.Stat(conf.Paths.BundleDefsRepo); err == nil {
		helpers.PrintBegin("pulling latest bundle definitions")
		err = helpers.PullRepo(conf.Paths.BundleDefsRepo)
		if err != nil {
			return err
		}
		helpers.PrintComplete("bundle repo pulled at %s", conf.Paths.BundleDefsRepo)
		return nil
	}
	helpers.PrintBegin("cloning latest bundle definitions")
	err := helpers.CloneRepo(url, filepath.Dir(conf.Paths.BundleDefsRepo))
	if err != nil {
		return err
	}
	helpers.PrintComplete("bundle repo cloned to %s", conf.Paths.BundleDefsRepo)
	return nil
}

func fetchUpdate(u uinfo) error {
	helpers.PrintBegin("fetching manifests from %s at version %d", u.url, u.ver)
	baseCache := filepath.Join(conf.Paths.CacheLocation, "update")
	outMoM := filepath.Join(baseCache, fmt.Sprint(u.ver), "Manifest.MoM")
	mom, err := helpers.DownloadManifest(u.url, u.ver, "MoM", outMoM)
	if err != nil {
		return err
	}

	for i := range mom.Files {
		ver := uint(mom.Files[i].Version)
		outMan := filepath.Join(baseCache, fmt.Sprint(ver), "Manifest."+mom.Files[i].Name)
		_, err := helpers.DownloadManifest(u.url, ver, mom.Files[i].Name, outMan)
		if err != nil {
			return err
		}
	}
	helpers.PrintComplete("manifests cached at %s", baseCache)
	return nil
}

func runFetchAllCmd(cmd *cobra.Command, args []string) {
	u, err := getUpstreamInfo(allFlags)
	if err != nil {
		helpers.Fail(err)
	}

	err = fetchRepo(u)
	if err != nil {
		helpers.Fail(err)
	}

	err = getLatestBundles(allFlags.bundleURL)
	if err != nil {
		helpers.Fail(err)
	}

	err = fetchUpdate(u)
	if err != nil {
		helpers.Fail(err)
	}
}

func runFetchBundlesCmd(cmd *cobra.Command, args []string) {
	err := getLatestBundles(allFlags.bundleURL)
	if err != nil {
		helpers.Fail(err)
	}
}

func runFetchRepoCmd(cmd *cobra.Command, args []string) {
	u, err := getUpstreamInfo(allFlags)
	if err != nil {
		helpers.Fail(err)
	}

	err = fetchRepo(u)
	if err != nil {
		helpers.Fail(err)
	}
}

func runFetchUpdateCmd(cmd *cobra.Command, args []string) {
	u, err := getUpstreamInfo(allFlags)
	if err != nil {
		helpers.Fail(err)
	}

	err = fetchUpdate(u)
	if err != nil {
		helpers.Fail(err)
	}
}
