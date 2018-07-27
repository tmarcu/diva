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
	"sort"
	"strings"
	"testing"

	"github.com/go-test/deep"
)

var core = Definition{
	Header: Header{
		Title: "os-core",
	},
	Includes: map[string]bool{},
	DirectPackages: map[string]bool{
		"bash-bin":           true,
		"ca-certs-static":    true,
		"clr-power-tweaks":   true,
		"clr-systemd-config": true,
		"util-linux-bin":     true,
	},
	AllPackages: map[string]bool{
		"bash-bin":           true,
		"ca-certs-static":    true,
		"clr-power-tweaks":   true,
		"clr-systemd-config": true,
		"util-linux-bin":     true,
	},
}

type testInstance struct {
	testdir string
	t       *testing.T
}

func newTestInstance(t *testing.T) testInstance {
	testData := testInstance{t: t}

	var err error
	testData.testdir, err = ioutil.TempDir("", "clr-bundles-")
	if err != nil {
		testData.t.Fatal(err)
	}

	// Create bundles/ within test directory
	if err := os.Mkdir(filepath.Join(testData.testdir, "bundles"), 0755); err != nil {
		testData.t.Fatal(err)
	}

	// Create packages file within test directory
	if _, err := os.Create(filepath.Join(testData.testdir, "packages")); err != nil {
		testData.t.Fatal(err)
	}

	// Create os-core bundle definition test file
	content := []string{"# [TITLE]: os-core",
		"# [DESCRIPTION]: Run a minimal Linux userspace", "# [STATUS]: Active",
		"# [CAPABILITIES]:", "# [MAINTAINER]: Super Smart Developer <email@email.com>",
		"bash-bin", "ca-certs-static", "clr-power-tweaks", "clr-systemd-config",
		"util-linux-bin"}

	testData.addBundle("os-core", "bundles/os-core", content...)

	return testData
}

// cleanup testdir and all of its content
func (testData *testInstance) teardown() {
	if err := os.RemoveAll(testData.testdir); err != nil {
		testData.t.Error(err)
	}
}

func (testData *testInstance) addBundle(name, dir string, content ...string) {
	// If the dir is bundles/ then overwrite any pre-existing file with the new content
	if dir != "packages" {
		err := ioutil.WriteFile(filepath.Join(testData.testdir, dir), []byte(strings.Join(content, "\n")), 0644)
		if err != nil {
			testData.t.Fatal(err)
		}
		return
	}

	// If the dir is the packages file, append content to the file, do not overwrite it.
	filemode := os.O_APPEND | os.O_CREATE | os.O_RDWR
	fname, err := os.OpenFile(filepath.Join(testData.testdir, dir), filemode, 0644)
	if err != nil {
		testData.t.Fatal(err)
	}

	if _, err := fname.WriteString(strings.Join(content, "\n")); err != nil {
		testData.t.Fatal(err)
	}

	defer func() {
		if err := fname.Close(); err != nil {
			testData.t.Error(err)
		}
	}()
}

func TestGetDefinitionValidHeaderPackageBundle(t *testing.T) {
	testData := newTestInstance(t)

	defer testData.teardown() // cleanup testdir

	name := "joe"
	content := []string{"first package", "second", name, "last-one"}
	testData.addBundle(name, "packages", content...)

	expectedHeader := Header{"joe", "", "", "", ""}

	actual, err := GetDefinition(name, testData.testdir)
	if err != nil {
		t.Fatal(err)
	}
	if expectedHeader != actual.Header {
		t.Fatal(fmt.Errorf("Bundle Header: %v\nDoes not match expected output: %v",
			actual.Header, expectedHeader))
	}
}

func TestGetDefinitionValidHeaderBundle(t *testing.T) {
	testData := newTestInstance(t)
	defer testData.teardown() // cleanup testdir

	testCases := []struct {
		bundleName     string
		testContent    []string
		expectedHeader Header
	}{
		{"dev-utils",
			[]string{
				"# [TITLE]: dev-utils",
				"# [DESCRIPTION]: Assist application development",
				"# [STATUS]: Active",
				"# [CAPABILITIES]",
				"# [MAINTAINER]: Developer Name <Developer@example.com>",
			},
			Header{
				"dev-utils",
				"Assist application development",
				"Active",
				"",
				"Developer Name <Developer@example.com>"},
		},
		{
			"dev-utils", // Header order or missing data in file should not matter
			[]string{
				"# [TITLE]: dev-utils",
				"# [DESCRIPTION]: Assist application development",
				"# [MAINTAINER]: Developer Name <Developer@example.com>",
			},
			Header{
				"dev-utils",
				"Assist application development",
				"",
				"",
				"Developer Name <Developer@example.com>"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.bundleName+" Validate Header", func(t *testing.T) {
			testData.addBundle(tc.bundleName, filepath.Join("bundles", tc.bundleName), tc.testContent...)
			actual, err := GetDefinition(tc.bundleName, testData.testdir)
			if err != nil {
				t.Fatal(err)
			}
			if tc.expectedHeader != actual.Header {
				t.Fatal(fmt.Errorf("Bundle Header: %v\nDoes not match expected output: %v",
					actual.Header, tc.expectedHeader))
			}
		})
	}
}

func TestBadHeaderItem(t *testing.T) {
	testData := newTestInstance(t)
	defer testData.teardown() // cleanup testdir

	name := "test"
	testContent := []string{
		"# [TITLE]: test",
		"# [DESCRIPTION]: Assist application development",
		"# [MAINTAINER]: Developer Name <Developer@example.com>",
		"# [RANDOMBADNESS]: What is this doing here",
	}

	testData.addBundle(name, filepath.Join("bundles", name), testContent...)
	_, err := GetDefinition(name, testData.testdir)
	if err.Error() != "Unknown header option found" {
		t.Fatalf("error %s did not match expected: 'Unknown header option found'", err.Error())
	}
}

func TestCorrectPackageBundleDefinition(t *testing.T) {
	testData := newTestInstance(t)
	defer testData.teardown() // cleanup testdir

	name := "xz"

	expected := Definition{
		Includes:       map[string]bool{"os-core": true, name: true},
		DirectPackages: map[string]bool{name: true},
		AllPackages:    map[string]bool{name: true},
	}
	for pkg := range core.AllPackages {
		expected.AllPackages[pkg] = true
	}

	testData.addBundle(name, "packages", name)

	actual, err := GetDefinition(name, testData.testdir)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(expected.Includes, actual.Includes) {
		t.Error(deep.Equal(expected.Includes, actual.Includes))
	}
	if !reflect.DeepEqual(expected.DirectPackages, actual.DirectPackages) {
		t.Error(deep.Equal(expected.DirectPackages, actual.DirectPackages))
	}
	if !reflect.DeepEqual(expected.AllPackages, actual.AllPackages) {
		t.Error(deep.Equal(expected.AllPackages, actual.AllPackages))
	}
}

func TestCorrectEntireBundleDefinitionKoji(t *testing.T) {
	testData := newTestInstance(t)
	defer testData.teardown() // cleanup testdir

	name := "koji"
	bundleAdds := []struct {
		name    string
		content []string
	}{
		{"koji",
			[]string{"include(package-utils)", "include(web-server-basic)",
				"koji", "koji-extras", "mash", "mod_wsgi", "nfs-utils", "postgresql"},
		},
		{"package-utils",
			[]string{"include(python3-basic)", "createrepo_c", "dnf", "mock"},
		},
		{"web-server-basic",
			[]string{"httpd", "nginx"},
		},
		{"python3-basic",
			[]string{"clr-python-timestamp", "glibc-lib-avx2", "virtualenv-python3"},
		},
	}

	for _, bundle := range bundleAdds {
		testData.addBundle(bundle.name, filepath.Join("bundles", bundle.name), bundle.content...)
	}

	expected := Definition{
		Includes: map[string]bool{"koji": true, "os-core": true,
			"package-utils": true, "python3-basic": true, "web-server-basic": true},
		DirectPackages: map[string]bool{"koji": true, "koji-extras": true,
			"mash": true, "mod_wsgi": true, "nfs-utils": true, "postgresql": true},
		AllPackages: map[string]bool{"clr-python-timestamp": true,
			"createrepo_c": true, "dnf": true, "glibc-lib-avx2": true, "mock": true,
			"httpd": true, "koji": true, "koji-extras": true, "mash": true,
			"virtualenv-python3": true, "mod_wsgi": true, "nfs-utils": true,
			"postgresql": true, "nginx": true},
	}

	for pkg := range core.AllPackages {
		expected.AllPackages[pkg] = true
	}

	actual, err := GetDefinition(name, testData.testdir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected.Includes, actual.Includes) {
		t.Error(deep.Equal(expected.Includes, actual.Includes))
	}
	if !reflect.DeepEqual(expected.DirectPackages, actual.DirectPackages) {
		t.Error(deep.Equal(expected.DirectPackages, actual.DirectPackages))
	}
	if !reflect.DeepEqual(expected.AllPackages, actual.AllPackages) {
		t.Error(deep.Equal(expected.AllPackages, actual.AllPackages))
	}
}

func TestWhitespaceBundleDefinitionCreation(t *testing.T) {
	testData := newTestInstance(t)
	defer testData.teardown() // cleanup testdir

	name := "package-utils"
	bundleAdds := []struct {
		name    string
		content []string
	}{
		{name,
			[]string{"# [TITLE]: package-utils",
				"# [DESCRIPTION]: Utilities for packages",
				"# [STATUS]: WIP",
				"", // empty string
				"# [CAPABILITIES]:",
				"include(python3-basic)",
				"# sad",                   // Out of order comment
				"include(python2-basic) ", // Trailing whitespace test
				"createrepo_c",
			},
		},
		{"python3-basic",
			[]string{},
		},
		{"python2-basic",
			[]string{"packageOne"},
		},
	}

	for _, bundle := range bundleAdds {
		testData.addBundle(bundle.name, filepath.Join("bundles", bundle.name), bundle.content...)
	}

	expected := Definition{
		Header: Header{
			"package-utils",
			"Utilities for packages",
			"WIP",
			"",
			"",
		},
		Includes: map[string]bool{"python3-basic": true, "python2-basic": true,
			"os-core": true, "package-utils": true},
		DirectPackages: map[string]bool{"createrepo_c": true},
		AllPackages:    map[string]bool{"createrepo_c": true, "packageOne": true},
	}

	for pkg := range core.AllPackages {
		expected.AllPackages[pkg] = true
	}

	actual, err := GetDefinition(name, testData.testdir)
	if err != nil {
		t.Fatal(err)
	}
	if expected.Header != actual.Header {
		t.Error("Header is not as expected.")
	}
	if !reflect.DeepEqual(expected.Includes, actual.Includes) {
		t.Error(deep.Equal(expected.Includes, actual.Includes))
	}
	if !reflect.DeepEqual(expected.DirectPackages, actual.DirectPackages) {
		t.Error(deep.Equal(expected.DirectPackages, actual.DirectPackages))
	}
	if !reflect.DeepEqual(expected.AllPackages, actual.AllPackages) {
		t.Error(deep.Equal(expected.AllPackages, actual.AllPackages))
	}
}

func TestNeitherBundleOrPackageBundle(t *testing.T) {
	testData := newTestInstance(t)
	defer testData.teardown() // cleanup testdir

	name := "Not-created-bundle"
	_, err := GetDefinition(name, testData.testdir)
	if err.Error() != fmt.Sprintf("%s is neither a pundle nor a bundle", name) {
		t.Fatalf("error %s did not match expected '%s is neither a pundle nor a bundle'", err.Error(), name)
	}
}

func TestCyclicalIncludes(t *testing.T) {
	testData := newTestInstance(t)
	defer testData.teardown() // cleanup testdir

	bundleAdds := []struct {
		name    string
		content []string
	}{
		{"cycle1",
			[]string{"include(cycle2)"},
		},
		{"cycle2",
			[]string{"include(cycle3)", "include(cycle4)"},
		},
		{"cycle3",
			[]string{"include(cycle4)"},
		},
		{"cycle4",
			[]string{"include(cycle5)"},
		},
		{"cycle5",
			[]string{"include(cycle3)"},
		},
	}

	for _, bundle := range bundleAdds {
		testData.addBundle(bundle.name, filepath.Join("bundles", bundle.name), bundle.content...)
	}

	_, err := GetDefinition("cycle2", testData.testdir)
	if strings.TrimSpace(err.Error()) != "Bundle include loop detected with cycle5 and cycle3" || err == nil {
		t.Fatalf("error %s did not match expected 'Bundle include loop detected'", err.Error())
	}
}

func TestCyclicalNeighboringIncludes(t *testing.T) {
	testData := newTestInstance(t)
	defer testData.teardown() // cleanup testdir

	bundleAdds := []struct {
		name    string
		content []string
	}{
		{"bundleA",
			[]string{"include(bundleB)", "include(bundleC)"},
		},
		{"bundleB",
			[]string{"include(bundleC)"},
		},
		{"bundleC",
			[]string{},
		},
	}

	for _, bundle := range bundleAdds {
		testData.addBundle(bundle.name, filepath.Join("bundles", bundle.name), bundle.content...)
	}

	_, err := GetDefinition("bundleA", testData.testdir)
	if err != nil {
		t.Fatalf("error bundle include loop exists when it shouldn't: %s.", err.Error())
	}
}

func TestAllBundlesSet(t *testing.T) {
	testData := newTestInstance(t)
	defer testData.teardown() // cleanup testdir

	bundleAdds := []struct {
		name    string
		content []string
	}{
		{"bundle1",
			[]string{"include(bundle2)", "include(pundle2)", "package1"},
		},
		{"bundle2",
			[]string{"package2", "package3"},
		},
		{"bundle3",
			[]string{"include(pundle1)", "package4"},
		},
	}

	for _, bundle := range bundleAdds {
		testData.addBundle(bundle.name, filepath.Join("bundles", bundle.name), bundle.content...)
	}

	for _, pundle := range []string{"pundle1\n", "pundle2\n", "pundle3\n"} {
		testData.addBundle(pundle, "packages", pundle)
	}

	bundleSet, err := GetAll(testData.testdir)
	if err != nil {
		t.Fatal(err)
	}

	// 6 definitions + os-core
	if len(bundleSet) != 7 {
		t.Fatal("Did not collect information on all bundles")
	}
}

func TestGetIncludesForBundle(t *testing.T) {
	testData := newTestInstance(t)
	defer testData.teardown() // cleanup testdir

	bundleAdds := []struct {
		name    string
		content []string
	}{
		{"koji",
			[]string{"include(package-utils)", "include(web-server-basic)",
				"koji", "koji-extras", "mash", "mod_wsgi", "nfs-utils", "postgresql"},
		},
		{"package-utils",
			[]string{"include(python3-basic)", "createrepo_c", "dnf", "mock"},
		},
		{"web-server-basic",
			[]string{"httpd", "nginx"},
		},
		{"python3-basic",
			[]string{"include(random-pundle)", "clr-python-timestamp", "glibc-lib-avx2", "virtualenv-python3"},
		},
	}

	testData.addBundle("random-pundle", "packages", "random-pundle")

	for _, bundle := range bundleAdds {
		testData.addBundle(bundle.name, filepath.Join("bundles", bundle.name), bundle.content...)
	}

	testCases := []struct {
		bundleName       string
		expectedIncludes []string
	}{
		{"koji",
			[]string{
				"koji", "os-core", "package-utils", "python3-basic", "random-pundle",
				"web-server-basic",
			},
		},
		{"package-utils",
			[]string{
				"os-core", "package-utils", "python3-basic", "random-pundle",
			},
		},
		{"web-server-basic",
			[]string{
				"os-core", "web-server-basic",
			},
		},
		{"python3-basic",
			[]string{
				"os-core", "python3-basic", "random-pundle",
			},
		},
		{"random-pundle",
			[]string{
				"os-core", "random-pundle",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.bundleName, func(t *testing.T) {
			actualIncludes, err := GetIncludesForBundle(tc.bundleName, testData.testdir)
			if err != nil || !reflect.DeepEqual(tc.expectedIncludes, actualIncludes) {
				t.Error(deep.Equal(tc.expectedIncludes, actualIncludes))
			}
		})
	}
}
