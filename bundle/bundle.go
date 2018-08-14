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

package bundle

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/clearlinux/diva/internal/helpers"
)

// Global variable to store os-core bundle definition
var coreBundle = &Definition{}

// Header is a struct that contains bundle header information.
type Header struct {
	Title        string
	Description  string
	Status       string
	Capabilities string
	Maintainer   string
}

// Definition stores bundle and pundle information. This includes the
// name, Header information, a set of bundle includes, a set of direct
// packages, and a set of all packages.
type Definition struct {
	Name   string
	Header Header

	Includes       map[string]bool
	DirectPackages map[string]bool
	AllPackages    map[string]bool
}

// Set is a map of bundle names to their definition
type Set map[string]*Definition

// Implement the global os-core bundle definition, that is used by all bundles
func initializeOsCore(bundlesDir string) error {
	var err error
	coreBundle, err = getBundleDefinition("os-core", bundlesDir, make(map[string]bool))
	return err
}

func newDefinition(name, bundlesDir string) (Definition, error) {
	// The os-core bundle must exist, and be incorporated into all bunde definitions
	if name != "os-core" && reflect.DeepEqual(coreBundle, &Definition{}) {
		if err := initializeOsCore(bundlesDir); err != nil {
			return Definition{}, err
		}
	}

	b := Definition{
		Includes:       make(map[string]bool),
		DirectPackages: make(map[string]bool),
		AllPackages:    make(map[string]bool),
	}

	b.Name = name
	b.Includes[name] = true

	for include := range coreBundle.Includes {
		b.Includes[include] = true
	}
	for pkg := range coreBundle.AllPackages {
		b.AllPackages[pkg] = true
	}

	return b, nil
}

func updateIncludes(packageInclude, bundlesDir string, b *Definition, visitedIncludes map[string]bool) error {
	if _, exists := visitedIncludes[packageInclude]; exists {
		return fmt.Errorf("Bundle include loop detected with %s and %s", b.Name, packageInclude)
	}

	visitedIncludes[packageInclude] = true

	include, err := getBundleDefinition(packageInclude, bundlesDir, visitedIncludes)
	if err != nil {
		return err
	}

	b.Includes[packageInclude] = true
	for inc := range include.Includes {
		b.Includes[inc] = true
	}
	for pkg := range include.AllPackages {
		b.AllPackages[pkg] = true
	}

	return nil
}

func readContent(name, bundlesDir string, b *Definition, visitedIncludes map[string]bool) (*Definition, error) {
	content, err := ioutil.ReadFile(filepath.Join(bundlesDir, "bundles", name))
	if err != nil {
		return nil, err
	}

	bundleHeaderFieldRegex := regexp.MustCompile(`^# \[([A-Z]+)\]:\s*(.*)$`)
	includeBundleRegex := regexp.MustCompile(`^include\(([A-Za-z0-9_-]+)\)$`)

	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if matches := bundleHeaderFieldRegex.FindStringSubmatch(line); len(matches) > 2 {
			key := matches[1]
			value := strings.TrimSpace(matches[2])
			switch key {
			case "TITLE":
				b.Header.Title = value
			case "DESCRIPTION":
				b.Header.Description = value
			case "STATUS":
				b.Header.Status = value
			case "CAPABILITIES":
				b.Header.Capabilities = value
			case "MAINTAINER":
				b.Header.Maintainer = value
			default:
				return nil, fmt.Errorf("Unknown header option found")
			}
			continue
		} else if matches := includeBundleRegex.FindStringSubmatch(line); len(matches) > 1 {
			err := updateIncludes(matches[1], bundlesDir, b, visitedIncludes)
			visitedIncludes = make(map[string]bool)
			if err != nil {
				return nil, err
			}
		} else {
			if line != "" && !strings.HasPrefix(line, "#") {
				b.DirectPackages[line] = true
				b.AllPackages[line] = true
			}
		}
	} // end reading file
	return b, nil
}

func checkIfPundle(name string, pundles string) bool {
	for _, line := range strings.Split(pundles, "\n") {
		if strings.EqualFold(line, name) {
			return true
		}
	}
	return false
}

func getPundleDefinition(name string, pundle *Definition) (*Definition, error) {
	pundle.Header.Title = name
	pundle.DirectPackages[name] = true
	pundle.AllPackages[name] = true
	return pundle, nil
}

func getBundleDefinition(name, bundlesDir string, visitedIncludes map[string]bool) (*Definition, error) {
	b, err := newDefinition(name, bundlesDir)
	if err != nil {
		return nil, err
	}

	if _, err = os.Stat(filepath.Join(bundlesDir, "bundles", name)); os.IsNotExist(err) {
		pundles, err := ioutil.ReadFile(filepath.Join(bundlesDir, "packages"))
		if err != nil {
			return nil, err
		}

		if isPundle := checkIfPundle(name, string(pundles)); isPundle {
			return getPundleDefinition(name, &b)
		}
		return nil, fmt.Errorf("%s is neither a pundle nor a bundle", name)
	}
	return readContent(name, bundlesDir, &b, visitedIncludes)
}

// GetDefinition reads the bundle definition from the bundlesDir repository and
// returns a *Definition of that bundle
func GetDefinition(name, bundlesDir string) (*Definition, error) {
	return getBundleDefinition(name, bundlesDir, make(map[string]bool))
}

// GetAll reads all bundle definitions in the bundlesDir repository and returns a
// map[string]*Definition of bundle names to their definition structs.
func GetAll(bundlesDir string) (Set, error) {
	bundles := make(Set)
	err := filepath.Walk(filepath.Join(bundlesDir, "bundles"), func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}
		bundle, err := GetDefinition(info.Name(), bundlesDir)
		if err != nil {
			return err
		}
		bundles[info.Name()] = bundle
		return nil
	})

	if err != nil {
		return nil, err
	}

	pundles, err := ioutil.ReadFile(filepath.Join(bundlesDir, "packages"))
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Split(string(pundles), "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}

		p, err := newDefinition(line, bundlesDir)
		if err != nil {
			return nil, err
		}

		pundle, err := getPundleDefinition(line, &p)
		if err != nil {
			return nil, err
		}
		bundles[line] = pundle
	}
	return bundles, nil
}

// GetIncludesForBundle returns a sorted slice of all includes for a specified bundle.
func GetIncludesForBundle(name, bundlesDir string) ([]string, error) {
	bundle, err := GetDefinition(name, bundlesDir)
	if err != nil {
		return nil, err
	}
	return helpers.HashmapToSortedSlice(bundle.Includes)
}

// GetAllPackagesForBundle returns a sorted slice of all packages for a specified
// bundle. AllPackages includes the direct packages for a bundle/pundle, along
// with the direct packages of the bundle includes; so all package dependencies.
func GetAllPackagesForBundle(name, bundlesDir string) ([]string, error) {
	bundle, err := GetDefinition(name, bundlesDir)
	if err != nil {
		return nil, err
	}
	return helpers.HashmapToSortedSlice(bundle.AllPackages)
}

// GetAllPackagesForAllBundles gets every package used by the bundle definitions.
// It returns a sorted slice of package names excluding any duplicates.
func GetAllPackagesForAllBundles(bundlesDir string) ([]string, error) {
	allPackages := make(map[string]bool)

	bundles, err := GetAll(bundlesDir)
	if err != nil {
		return nil, err
	}

	for _, bundle := range bundles {
		for p := range bundle.AllPackages {
			allPackages[p] = true
		}
	}
	return helpers.HashmapToSortedSlice(allPackages)
}
