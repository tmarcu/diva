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

package download

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/clearlinux/diva/internal/helpers"
	"github.com/clearlinux/diva/pkginfo"
	"github.com/clearlinux/mixer-tools/swupd"
)

// GetManifest downloads a manifest to outF
func GetManifest(baseURL string, version string, component, outF string) error {
	if _, err := os.Lstat(outF); err == nil {
		return nil
	}
	url := fmt.Sprintf("%s/update/%s/Manifest.%s.tar", baseURL, version, component)

	err := os.MkdirAll(filepath.Dir(outF), 0744)
	if err != nil {
		return err
	}
	err = helpers.TarExtractURL(url, outF)
	if err != nil {
		return err
	}

	return nil
}

// GetMom returns the downloaded and parsed swupd.manifest mom object
func GetMom(mInfo pkginfo.ManifestInfo) (*swupd.Manifest, error) {
	outMoM := filepath.Join(filepath.Join(mInfo.CacheLoc, "update"), mInfo.Version, "Manifest.MoM")
	err := GetManifest(mInfo.UpstreamURL, mInfo.Version, "MoM", outMoM)
	if err != nil {
		return nil, err
	}
	return swupd.ParseManifestFile(outMoM)
}

// UpdateContent downloads all manifest from the MOM file
func UpdateContent(mInfo *pkginfo.ManifestInfo) error {
	var err error

	baseCache := filepath.Join(mInfo.CacheLoc, "update")
	outMoM := filepath.Join(baseCache, mInfo.Version, "Manifest.MoM")
	err = GetManifest(mInfo.UpstreamURL, mInfo.Version, "MoM", outMoM)
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
		err := GetManifest(mInfo.UpstreamURL, mInfo.Version, mom.Files[i].Name, outMan)
		if err != nil {
			return err
		}
	}
	return nil
}
