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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/clearlinux/diva/internal/config"
	"github.com/clearlinux/diva/internal/helpers"
	"github.com/clearlinux/diva/pkginfo"
	"github.com/clearlinux/mixer-tools/swupd"
)

// FetchRepo fetches the RPM repo at the u.URL baseurl to the local cache
// location
func FetchRepo(conf *config.Config, u *config.UInfo) error {

	repo, err := pkginfo.NewRepo(conf, u)
	if err != nil {
		return err
	}

	helpers.PrintBegin("fetching repo from %s", repo.URI)
	err = pkginfo.DownloadRepoFiles(&repo, u.Update)
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
	err = pkginfo.DownloadBundles(&bundleInfo)
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
	baseCache := filepath.Join(mInfo.CacheLoc, "update")
	outMoM := filepath.Join(baseCache, mInfo.Version, "Manifest.MoM")
	err = helpers.DownloadManifest(mInfo.UpstreamURL, mInfo.Version, "MoM", outMoM)
	if err != nil {
		return err
	}
	mom, err := swupd.ParseManifestFile(outMoM)
	if err != nil {
		return err
	}

	for i := range mom.Files {
		ver := uint(mom.Files[i].Version)
		if ver < mInfo.MinVer {
			continue
		}
		outMan := filepath.Join(baseCache, fmt.Sprint(ver), "Manifest."+mom.Files[i].Name)
		err := helpers.DownloadManifest(mInfo.UpstreamURL, mInfo.Version, mom.Files[i].Name, outMan)
		if err != nil {
			return err
		}
	}
	helpers.PrintComplete("manifests cached at %s", baseCache)
	return nil
}

type finfo struct {
	out string
	url string
	err error
}

func getAllManifests(mInfo pkginfo.ManifestInfo) (map[string]finfo, error) {
	dlFiles := make(map[string]finfo)
	baseCache := filepath.Join(mInfo.CacheLoc, "update")
	outMoM := filepath.Join(baseCache, mInfo.Version, "Manifest.MoM")
	err := helpers.DownloadManifest(mInfo.UpstreamURL, mInfo.Version, "MoM", outMoM)
	if err != nil {
		return nil, err
	}

	mom, err := swupd.ParseManifestFile(outMoM)
	if err != nil {
		return nil, err
	}

	// this is fast, no need to parallelize
	for i := range mom.Files {
		mv := uint(mom.Files[i].Version)
		if mv < mInfo.MinVer {
			continue
		}
		outMan := filepath.Join(baseCache, fmt.Sprint(mv), "Manifest."+mom.Files[i].Name)
		err := helpers.DownloadManifest(mInfo.UpstreamURL, fmt.Sprint(mv), mom.Files[i].Name, outMan)
		if err != nil {
			return nil, err
		}

		m, err := swupd.ParseManifestFile(outMan)
		if err != nil {
			return nil, err
		}

		for _, f := range m.Files {
			if uint(f.Version) < mInfo.MinVer || !f.Present() {
				continue
			}

			fURL := fmt.Sprintf("%s/update/%d/files/%s.tar", mInfo.UpstreamURL, f.Version, f.Hash)
			fOut := filepath.Join(baseCache, fmt.Sprint(f.Version), "files", f.Hash.String()+".tar")
			fi := finfo{out: fOut, url: fURL}
			dlFiles[fOut] = fi
		}
	}
	return dlFiles, nil
}

// FetchUpdateFiles downloads relevant files for u.Ver from u.URL
func FetchUpdateFiles(conf *config.Config, u *config.UInfo) error {

	mInfo, err := pkginfo.NewManifestInfo(conf, u)
	if err != nil {
		return err
	}

	helpers.PrintBegin("fetching files from %s at version %v", mInfo.UpstreamURL, mInfo.Version)
	dlFiles, err := getAllManifests(mInfo)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	nworkers := 8
	wg.Add(nworkers)
	fChan := make(chan finfo)
	errChan := make(chan error, nworkers)

	for i := 0; i < nworkers; i++ {
		go func() {
			defer wg.Done()
			for f := range fChan {
				// we already have this file cached
				if _, err := os.Lstat(strings.TrimSuffix(f.out, ".tar")); err == nil {
					continue
				}

				f.err = helpers.TarExtractURL(f.url, f.out)
				_ = os.Remove(f.out)

				if f.err != nil {
					errChan <- f.err
				}
			}
		}()
	}

	for f := range dlFiles {
		fChan <- dlFiles[f]
	}
	close(fChan)
	wg.Wait()

	if len(errChan) > 0 {
		helpers.PrintComplete("errors downloading %d files", len(errChan))
	} else {
		helpers.PrintComplete("files cached at %s", filepath.Join(mInfo.CacheLoc, "update"))
	}
	return nil
}
