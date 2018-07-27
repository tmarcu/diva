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

// Repo defines the location, name, type, and other metadata about an RPM
// repository, as well as a slice of pointers to RPMs
type Repo struct {
	URI      string
	Name     string
	Version  string
	Type     string
	CacheDir string
	Priority uint
	Packages []*RPM
}

// RPM is a packaging format that encapsulates a collection of files to install
// and assorted metadata. An RPM can be either a binary or source RPM. If
// SRPMName is empty this indicates the RPM is already a source RPM. For binary
// RPMs it will be populated with that RPMs associated source RPM name.
type RPM struct {
	Name          string
	Version       string
	Release       string
	Architecture  string
	SRPMName      string
	License       string
	Requires      []string
	BuildRequires []string
	Provides      []string
	Files         []*File
}

// File contains all information for a file in an RPM.
// Additional fields Name, Type, SwupdHash, and CurrentVersion are used by
// swupd operations.
type File struct {
	Name           string
	Type           byte
	Size           uint
	Hash           string
	SwupdHash      string
	Permissions    string
	Owner          string
	Group          string
	SymlinkTarget  string
	CurrentVersion uint
}
