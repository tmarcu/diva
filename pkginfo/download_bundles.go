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

package pkginfo

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/clearlinux/diva/internal/helpers"
)

// DownloadBundles clones or pulls the latest clr-bundles definitions to
// to the desired cache location and checking out the version tag. It also
// modifies the bundleInfo object by storing the current branch name for future
// use and cleanup.
func DownloadBundles(bundleInfo *BundleInfo) error {
	var err error

	if _, err = os.Stat(bundleInfo.BundleCache); err != nil {
		err = helpers.CloneRepo(bundleInfo.BundleURL, filepath.Dir(bundleInfo.BundleCache))
		if err != nil {
			return err
		}
	}

	// Get the name of the current branch to defer back to
	bundleInfo.Branch, err = GetCurrentBranch(bundleInfo.BundleCache)
	if err != nil {
		return err
	}

	// ensure repo up to date
	err = helpers.PullRepo(bundleInfo.BundleCache)
	if err != nil {
		return err
	}

	return helpers.CheckoutRepoTag(bundleInfo.BundleCache, bundleInfo.Version)
}

// GetCurrentBranch gets the current branch the bundles repo is on, so that
// if a different tag is checked out, and it is in detached HEAD state a
// defer to checkout what the current branch the repo is on will fix the state.
func GetCurrentBranch(repoPath string) (string, error) {
	args := []string{"-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD"}
	branch, err := helpers.RunCommandOutput("git", args...)
	return strings.TrimSpace(branch.String()), err
}
