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
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	rpm "github.com/cavaliercoder/go-rpm"
	"github.com/clearlinux/diva/internal/config"
	"github.com/clearlinux/diva/internal/helpers"
)

// config object used by GetUpstreamRepoFiles and called functions
var c *config.Config

// buildFileListsURL parses upstream repomd.xml file to find the filelists
// file.  We cannot just look for the filelists file directly because a hash is
// part of the filename. The repomd.xml file lists this file name so we can
// construct the url using this value.
func buildFilelistsURL(repo *Repo, workingDir string, update bool) (string, error) {
	// download repomd.xml
	repomdFile := filepath.Join(workingDir, "repomd.xml")
	repomdURL := fmt.Sprintf("%s/repodata/repomd.xml", repo.URI)

	var err error
	if !update {
		_, err = os.Stat(repomdFile)
	}
	if err != nil || update {
		err = helpers.Download(repomdURL, repomdFile, update)
		if err != nil {
			return "", err
		}
	}

	// <repomd>
	//   <data type="name">
	//     <location href="url-extension"/>
	//   </data>
	//   ...
	//   </repomd>
	type loc struct {
		Path string `xml:"href,attr"`
	}
	type data struct {
		Key      string `xml:"type,attr"`
		Location loc    `xml:"location"`
	}
	type repomd struct {
		XMLName xml.Name `xml:"repomd"`
		Data    []data   `xml:"data"`
	}

	d, err := ioutil.ReadFile(repomdFile)
	if err != nil {
		return "", err
	}
	v := new(repomd)
	err = xml.Unmarshal([]byte(d), v)
	if err != nil {
		return "", err
	}

	var path string
	for _, section := range v.Data {
		if section.Key == "filelists" {
			path = section.Location.Path
			break
		}
	}

	return fmt.Sprintf("%s/%s", repo.URI, path), nil
}

// If a version changed from what was in the cache, return true, otherwise false
func versionChanged(rpmURL, cacheDir string, cRPM *rpm.PackageFile) bool {
	newRPM, err := rpm.OpenPackageFile(rpmURL)
	if err != nil {
		return true
	}
	// VersionCompare returns 1 if newRPM is more recent, -1 if cRPM is more recent
	// and 0, if they're the same.
	return rpm.VersionCompare(newRPM, cRPM) == 1
}

// If a RPM needs to be redownloaded, return true, otherwise return false
func updateCache(cachedRPMs []*rpm.PackageFile, rpmName, rpmURL, cacheDir string) bool {
	for _, cRPM := range cachedRPMs {
		if cRPM.Name() == rpmName {
			if !versionChanged(rpmURL, cacheDir, cRPM) {
				return false
			}
			staleRPM := fmt.Sprintf("%s/%s-%s-%s.%s.rpm",
				cacheDir,
				cRPM.Name(),
				cRPM.Version(),
				cRPM.Release(),
				cRPM.Architecture(),
			)
			if err := os.Remove(staleRPM); err != nil {
				return true
			}
		}
	}
	return true
}

func buildPackageURLs(repo *Repo, flistsPath, workingDir string, update bool) ([]string, error) {
	// <filelists xmlns="http://linux.duke.edu/metadata/filelists" packages="7436">
	//   <package ... name="pkgname" arch="x86_64">
	//     <version ... ver="ver" rel="rel"/>
	//     ...
	//   </package>
	type verRel struct {
		V string `xml:"ver,attr"`
		R string `xml:"rel,attr"`
	}
	type packageInfo struct {
		Name string `xml:"name,attr"`
		VR   verRel `xml:"version"`
		Arch string `xml:"arch,attr"`
	}
	type filelists struct {
		XMLName  xml.Name      `xml:"filelists"`
		Packages []packageInfo `xml:"package"`
	}

	d, err := ioutil.ReadFile(flistsPath)
	if err != nil {
		return []string{}, err
	}

	v := new(filelists)
	err = xml.Unmarshal(d, v)
	if err != nil {
		return []string{}, err
	}

	cacheDir := filepath.Join(workingDir, "packages")
	if err = os.MkdirAll(cacheDir, 0755); err != nil {
		return []string{}, err
	}
	cachedRPMs, err := rpm.OpenPackageFiles(cacheDir)
	if err != nil {
		return []string{}, err
	}

	packages := []string{}
	url := fmt.Sprintf("%s/Packages", repo.URI)
	for _, p := range v.Packages {
		rpmURL := fmt.Sprintf("%s/%s-%s-%s.%s.rpm", url, p.Name, p.VR.V, p.VR.R, p.Arch)
		if update {
			if !updateCache(cachedRPMs, p.Name, rpmURL, cacheDir) {
				continue
			}
		}
		packages = append(packages, rpmURL)
	}

	return packages, nil
}

func downloadAllRPMs(packages []string, workingDir string) error {
	// ensure directory in cache exists
	outPath := filepath.Join(workingDir, "packages")
	if err := os.MkdirAll(outPath, 0755); err != nil {
		return err
	}
	var wg sync.WaitGroup
	workers := runtime.NumCPU()
	wg.Add(workers)
	urlCh := make(chan string)
	errorCh := make(chan error)

	// download worker
	dlWorker := func() {
		for url := range urlCh {
			base := filepath.Base(url)
			outFile := filepath.Join(outPath, base)
			var dlErr error
			// do not download again if it already exists
			if _, dlErr = os.Stat(outFile); dlErr != nil {
				dlErr = helpers.Download(url, outFile, false)
			}
			if dlErr != nil {
				// report the error to the user
				errorCh <- dlErr
			}
		}
		wg.Done()
	}

	// collect errors as they happen so we can report the number of errors
	// at the end.
	errSummary := make(chan []error)
	go func() {
		var errs []error
		for dlErr := range errorCh {
			errs = append(errs, dlErr)
		}
		errSummary <- errs
	}()

	// kick off the dlWorkers
	for i := 0; i < workers; i++ {
		go dlWorker()
	}

	// populate the url channel
	for _, url := range packages {
		urlCh <- url
	}
	close(urlCh)
	wg.Wait()
	// close this when all the urls have finished processing
	close(errorCh)

	// final check for error that could happen after all workers are spawned
	// report failed downloads to user
	errors := <-errSummary
	if len(errors) > 0 {
		return fmt.Errorf("unable to download %d RPMs, please try again", len(errors))
	}

	return nil
}

// DownloadRepoFiles downloads all RPM packages from the RPM repo at the given
// baseURL by first parsing the repo metadata. These packages are downloaded to
// the c.CacheLocation/rpms/<version>/packages/ if they do not already exist
// there.
func DownloadRepoFiles(repo *Repo, update bool) (string, error) {
	var err error
	c, err = config.ReadConfig("")
	if err != nil {
		return "", err
	}

	workingDir := filepath.Join(
		c.Paths.CacheLocation,
		"rpms",
		repo.Name,
		repo.Version,
		repo.Type,
	)

	if err = os.MkdirAll(workingDir, 0755); err != nil {
		return "", err
	}

	url, err := buildFilelistsURL(repo, workingDir, update)
	if err != nil {
		return "", err
	}

	flistsPath := filepath.Join(workingDir, "filelists.xml")
	// this file can be either gz or xz compressed, use DownloadFile
	// which will use whatever extraction method is appropriate based
	// on the file extension.
	err = helpers.DownloadFile(url, flistsPath, update)
	if err != nil {
		return "", err
	}

	packages, err := buildPackageURLs(repo, flistsPath, workingDir, update)
	if err != nil {
		return "", err
	}

	return filepath.Join(workingDir, "packages"), downloadAllRPMs(packages, workingDir)
}
