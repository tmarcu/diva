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

package diva

import (
	"github.com/clearlinux/diva/download"
	"github.com/clearlinux/diva/internal/config"
	"github.com/clearlinux/diva/internal/helpers"
	"github.com/clearlinux/diva/pkginfo"
)

// FetchRepo fetches the RPM repo at the u.URL baseurl to the local cache
// location
func FetchRepo(conf *config.Config, u *config.UInfo) error {

	repo, err := pkginfo.NewRepo(conf, u)
	if err != nil {
		return err
	}

	helpers.PrintBegin("fetching repo from %s", repo.URI)
	err = download.RepoFiles(&repo, u.Update)
	if err != nil {
		return err
	}

	err = pkginfo.ImportAllRPMs(&repo, u.Update)
	if err != nil {
		return err
	}

	helpers.PrintComplete("repo cached at %s", repo.RPMCache)
	return nil
}

// FetchBundles clones the bundles repository from the config or passed in
// bundleURL argument and imports the information to the database.
func FetchBundles(conf *config.Config, u *config.UInfo) error {

	bundleInfo, err := pkginfo.NewBundleInfo(conf, u)
	if err != nil {
		return err
	}

	helpers.PrintBegin("getting bundle definitions")
	err = download.Bundles(&bundleInfo)
	if err != nil {
		return err
	}
	helpers.PrintComplete("bundle repo cached to %s", bundleInfo.BundleCache)

	err = pkginfo.ImportBundleDefinitions(&bundleInfo)
	if err != nil {
		return err
	}

	// after fetching from the specified tag, defer back to previous state
	defer func() {
		_ = helpers.CheckoutBranch(bundleInfo.BundleCache, bundleInfo.Branch)
	}()

	return nil
}

// FetchUpdate downloads manifests from the u.URL server
func FetchUpdate(conf *config.Config, u *config.UInfo) error {
	mInfo, err := pkginfo.NewManifestInfo(conf, u)
	if err != nil {
		return err
	}

	helpers.PrintBegin("fetching manifests from %s at version %v", mInfo.UpstreamURL, mInfo.Version)
	err = download.UpdateContent(&mInfo)
	if err != nil {
		return err
	}
	helpers.PrintComplete("manifests cached at %s", mInfo.CacheLoc)
	return nil
}

// FetchUpdateFiles downloads relevant files for u.Ver from u.URL
func FetchUpdateFiles(conf *config.Config, u *config.UInfo) error {
	mInfo, err := pkginfo.NewManifestInfo(conf, u)
	if err != nil {
		return err
	}

	helpers.PrintBegin("fetching manifests files from %s at version %v", mInfo.UpstreamURL, mInfo.Version)
	err = download.UpdateFiles(&mInfo)
	if err != nil {
		return err
	}
	helpers.PrintComplete("manifests cached at %s/update", mInfo.CacheLoc)
	return nil
}
