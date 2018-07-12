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

package bloatcheck

import (
	"fmt"
	"sort"
	"sync"

	"github.com/clearlinux/diva/bundle"
	"github.com/clearlinux/diva/diva"
	"github.com/clearlinux/mixer-tools/swupd"

	"path/filepath"
)

var sizeMutex sync.RWMutex

func getManifests(u diva.UInfo) ([]*swupd.Manifest, error) {
	baseCache := filepath.Join(u.CacheLoc, "update")
	momPath := filepath.Join(baseCache, fmt.Sprint(u.Ver), "Manifest.MoM")

	mom, err := swupd.ParseManifestFile(momPath)
	if err != nil {
		return nil, err
	}

	var path string
	var manifests []*swupd.Manifest
	for _, manifest := range mom.Files {

		path = filepath.Join(baseCache, fmt.Sprint(manifest.Version), "Manifest."+manifest.Name)
		mf, err := swupd.ParseManifestFile(path)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, mf)
	}
	return manifests, nil
}

func findManifest(name string, fs []*swupd.File) *swupd.File {
	i := sort.Search(len(fs), func(i int) bool {
		return fs[i].Name >= name
	})

	if i < len(fs) && fs[i].Name == name {
		return fs[i]
	}

	return nil
}

// Concurrently gets the size for each manifest
func getSizes(u diva.UInfo, m *swupd.Manifest, mom *swupd.Manifest, bundleSizes map[string]int64, bundlePath string) error {
	if m.Name == "os-core-update-index" {
		return nil
	}

	includes, err := bundle.GetIncludesForBundle(m.Name, bundlePath)
	if err != nil {
		fmt.Printf("Failed to get includes for manifest %s\n", m.Name)
		return err
	}

	baseCache := filepath.Join(u.CacheLoc, "update")
	for _, i := range includes {
		manifestFile := findManifest(i, mom.Files)
		if manifestFile == nil {
			return err
		}
		path := filepath.Join(baseCache, fmt.Sprint(manifestFile.Version), "Manifest."+i)
		includeManifest, err := swupd.ParseManifestFile(path)
		if err != nil {
			return err
		}
		sizeMutex.Lock()
		bundleSizes[m.Name] += int64(includeManifest.Header.ContentSize)
		sizeMutex.Unlock()
	}
	sizeMutex.Lock()
	bundleSizes[m.Name] += int64(m.Header.ContentSize)
	sizeMutex.Unlock()

	return nil
}

// GetBundleSize gets the full size of all bundles in a given version
func GetBundleSize(u diva.UInfo, bundlePath string) (map[string]int64, error) {
	manifests, err := getManifests(u)

	var wg sync.WaitGroup
	bundleWorkers := len(manifests)
	mChan := make(chan *swupd.Manifest)
	errChan := make(chan error, bundleWorkers)
	wg.Add(bundleWorkers)

	var bundleSizes = make(map[string]int64)

	baseCache := filepath.Join(u.CacheLoc, "update")
	momPath := filepath.Join(baseCache, fmt.Sprint(u.Ver), "Manifest.MoM")

	mom, err := swupd.ParseManifestFile(momPath)
	if err != nil {
		return nil, err
	}

	sort.Slice(mom.Files, func(i, j int) bool {
		return mom.Files[i].Name < mom.Files[j].Name
	})

	for i := 0; i < bundleWorkers; i++ {
		go func() {
			defer wg.Done()
			for m := range mChan {
				// Get the total size for each bundle (without accounting for overlap between them)
				err = getSizes(u, m, mom, bundleSizes, bundlePath)
				if err != nil {
					errChan <- err
				}
			}
		}()
	}

	for _, m := range manifests {
		select {
		case mChan <- m:
		case err = <-errChan:
			break
		}
	}
	close(mChan)
	wg.Wait()

	chanLen := len(errChan)
	for i := 0; i < chanLen; i++ {
		<-errChan
	}

	return bundleSizes, err
}
