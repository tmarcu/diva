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
	"github.com/clearlinux/diva/diva"
	"github.com/clearlinux/diva/internal/config"
	"github.com/clearlinux/diva/internal/helpers"

	"github.com/spf13/cobra"
)

type allFetchFlags struct {
	mixName         string
	version         string
	latest          bool
	upstreamURL     string
	rpmType         string
	upstreamRepoURL string
	rpmCache        string
	bundleURL       string
	bundleCache     string
	update          bool
	recursive       bool
}

var fetchFlags allFetchFlags

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch and cache various build information",
}

var fetchAllCmd = &cobra.Command{
	Use:   "all [--version <version>] [--bundleurl|--upstreamurl <url>] [--update]",
	Run:   runFetchAllCmd,
	Short: "Fetch metadata from <version> or latest available if <version> is not supplied",
	Long: `Fetch latest bundle definitions, RPM repository, and update manifests from <version>.
If <version> is not supplied, fetch latest available content. Pass
--upstreamurl to fetch RPMs and update metadata from a location other than the
configured Upstream URL and --bundleurl to fetch the bundle definitions from a
location other than default. If --update is passed, the cached Repo data will
be updated with new information from the upstream url.

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

	fetchCmd.PersistentFlags().StringVarP(&fetchFlags.mixName, "name", "n", "clear", "optional name of data group")
	fetchCmd.PersistentFlags().StringVarP(&fetchFlags.version, "version", "v", "0", "version from which to pull data")
	fetchCmd.PersistentFlags().StringVarP(&fetchFlags.upstreamURL, "upstreamurl", "u", "", "URL from which to pull update metadata")
	fetchCmd.PersistentFlags().BoolVar(&fetchFlags.latest, "latest", false, "get the latest upstream version")

	fetchAllCmd.Flags().StringVarP(&fetchFlags.upstreamRepoURL, "repourl", "m", "", "fully qualified URL from which to pull repodata")
	fetchAllCmd.Flags().StringVar(&fetchFlags.rpmCache, "rpmcache", "", "path to repo cache destination")
	fetchAllCmd.Flags().StringVar(&fetchFlags.rpmType, "rpmtype", "B", "type of rpm")
	fetchAllCmd.Flags().StringVarP(&fetchFlags.bundleURL, "bundleurl", "b", "", "URL from which to pull bundle definitions")
	fetchAllCmd.Flags().StringVar(&fetchFlags.bundleURL, "bundlecache", "", "path to bundle cache destination")
	fetchAllCmd.Flags().BoolVar(&fetchFlags.update, "update", false, "update pre-existing Repo data")
	fetchAllCmd.Flags().BoolVarP(&fetchFlags.recursive, "recursive", "r", false, "recursively fetch all content referenced in update metadata")

	fetchRepoCmd.Flags().StringVarP(&fetchFlags.upstreamRepoURL, "repourl", "m", "", "fully qualified URL from which to pull repodata")
	fetchRepoCmd.Flags().StringVar(&fetchFlags.rpmCache, "rpmcache", "", "path to repo cache destination")
	fetchRepoCmd.Flags().StringVar(&fetchFlags.rpmType, "rpmtype", "B", "type of rpm")
	fetchRepoCmd.Flags().BoolVar(&fetchFlags.update, "update", false, "update data with upstream")

	fetchBundlesCmd.Flags().StringVarP(&fetchFlags.bundleURL, "bundleurl", "b", "", "URL from which to pull bundle definitions")
	fetchBundlesCmd.Flags().StringVar(&fetchFlags.bundleURL, "bundlecache", "", "path to bundle cache destination")

	fetchUpdateCmd.Flags().BoolVarP(&fetchFlags.recursive, "recursive", "r", false, "recursively fetch all content referenced in update metadata")
}

func newUinfo() config.UInfo {
	u := config.UInfo{
		Ver:         fetchFlags.version,
		Latest:      fetchFlags.latest,
		URL:         fetchFlags.upstreamURL,
		MixName:     fetchFlags.mixName,
		Update:      fetchFlags.update,
		RepoURL:     fetchFlags.upstreamRepoURL,
		RPMCache:    fetchFlags.rpmCache,
		RPMType:     fetchFlags.rpmType,
		BundleURL:   fetchFlags.bundleURL,
		BundleCache: fetchFlags.bundleCache,
	}
	config.UpdateConfigInstance(conf, u)
	return u
}

func runFetchAllCmd(cmd *cobra.Command, args []string) {
	var err error
	u := newUinfo()

	err = diva.FetchRepo(conf, &u)
	helpers.FailIfErr(err)

	err = diva.FetchBundles(conf, &u)
	helpers.FailIfErr(err)

	err = diva.FetchUpdate(conf, &u)
	helpers.FailIfErr(err)
}

func runFetchRepoCmd(cmd *cobra.Command, args []string) {
	var err error
	u := newUinfo()

	err = diva.FetchRepo(conf, &u)
	helpers.FailIfErr(err)
}

func runFetchBundlesCmd(cmd *cobra.Command, args []string) {
	var err error
	u := newUinfo()

	err = diva.FetchBundles(conf, &u)
	helpers.FailIfErr(err)
}

func runFetchUpdateCmd(cmd *cobra.Command, args []string) {
	var err error
	u := newUinfo()

	err = diva.FetchUpdate(conf, &u)
	helpers.FailIfErr(err)

	err = diva.FetchUpdateFiles(conf, &u)
	helpers.FailIfErr(err)
}
