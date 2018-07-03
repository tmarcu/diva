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
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/clearlinux/diva/internal/config"
	"github.com/clearlinux/diva/internal/helpers"
	"github.com/clearlinux/diva/pkginfo"
	"github.com/clearlinux/mixer-tools/swupd"
)

// UInfo describes basic information about the upstream update server and local
// cache location
type UInfo struct {
	Ver      string
	URL      string
	CacheLoc string
}

// GetUpstreamInfo populates the UInfo struct and returns it
func GetUpstreamInfo(conf *config.Config, upstreamURL string, version string) (UInfo, error) {
	u := UInfo{}
	if upstreamURL == "" {
		u.URL = conf.UpstreamURL
	} else {
		u.URL = upstreamURL
	}

	if version != "" {
		u.Ver = version
		// no need to continue
		return u, nil
	}

	resp, err := http.Get(u.URL + "/latest")
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

	u.Ver = strings.Trim(string(body), "\n")
	// no support yet for modifying cache location via commandline
	u.CacheLoc = conf.Paths.CacheLocation
	return u, err
}

// FetchRepo fetches the RPM repo at the u.URL baseurl to the local cache
// location
func FetchRepo(u UInfo) error {
	repo := &pkginfo.Repo{
		URI:     fmt.Sprintf("%s/releases/%s/clear/x86_64/os/", u.URL, u.Ver),
		Name:    "clear",
		Version: u.Ver,
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

// GetLatestBundles clones or pulls the latest clr-bundles definitions to
// conf.Paths.BundleDefsRepo
func GetLatestBundles(conf *config.Config, url string) error {
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

// FetchUpdate downloads manifests from the u.URL server
func FetchUpdate(u UInfo) error {
	helpers.PrintBegin("fetching manifests from %s at version %s", u.URL, u.Ver)
	baseCache := filepath.Join(u.CacheLoc, "update")
	outMoM := filepath.Join(baseCache, u.Ver, "Manifest.MoM")
	err := helpers.DownloadManifest(u.URL, u.Ver, "MoM", outMoM)
	if err != nil {
		return err
	}
	mom, err := swupd.ParseManifestFile(outMoM)
	if err != nil {
		return err
	}

	for i := range mom.Files {
		ver := fmt.Sprint(mom.Files[i].Version)
		outMan := filepath.Join(baseCache, ver, "Manifest."+mom.Files[i].Name)
		err := helpers.DownloadManifest(u.URL, ver, mom.Files[i].Name, outMan)
		if err != nil {
			return err
		}
	}
	helpers.PrintComplete("manifests cached at %s", baseCache)
	return nil
}
