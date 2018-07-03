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
	"fmt"
	"testing"

	"github.com/rafaeljusto/redigomock"
)

func TestGetRepoRedis(t *testing.T) {
	repo := &Repo{
		Name:    "testrepo",
		Version: "100",
		Type:    "B",
	}
	repoKey := fmt.Sprintf("%s%s%s", repo.Name, repo.Version, repo.Type)
	pkgsKey := fmt.Sprintf("%s:packages", repoKey)
	pkgKey := fmt.Sprintf("%s:testpkg", repoKey)
	fIdxKey := fmt.Sprintf("%s:files", pkgKey)
	fKey := fmt.Sprintf("%s:file", pkgKey)

	conn := redigomock.NewConn()
	cmds := []*redigomock.Cmd{
		conn.Command("SMEMBERS", pkgsKey).ExpectStringSlice("testpkg"),
		// this effectively tests getRPMRedis as well
		conn.Command("HGET", pkgKey, "Name").Expect("testpkg"),
		conn.Command("HGET", pkgKey, "Version").Expect("100"),
		conn.Command("HGET", pkgKey, "Release").Expect("1"),
		conn.Command("HGET", pkgKey, "Architecture").Expect("xTEST"),
		conn.Command("HGET", pkgKey, "SRPMName").Expect("testpkg.src.rpm"),
		conn.Command("HGET", pkgKey, "License").Expect("license"),
		conn.Command("HGET", pkgKey, "Requires").Expect([]byte("reqs")),
		conn.Command("HGET", pkgKey, "BuildRequires").Expect([]byte("breqs")),
		conn.Command("HGET", pkgKey, "Provides").Expect([]byte("provs")),
		// this effectively tests getFilesRedis as well
		conn.Command("HVALS", fIdxKey).ExpectStringSlice([]string{"file1", "file2"}...),
		conn.Command("HGETALL", fKey+"1").ExpectMap(map[string]string{"Name": "f1"}),
		conn.Command("HGETALL", fKey+"2").ExpectMap(map[string]string{"Name": "f2"}),
	}
	err := getRepoRedis(conn, repo)
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range cmds {
		if conn.Stats(c) == 0 {
			t.Errorf("expected command %s %s was not called", c.Name, c.Args)
		}
	}

	if len(repo.Packages) != 1 {
		// fatal since we access via indices below
		t.Fatalf("expected 1 package but got %d", len(repo.Packages))
	}

	p := repo.Packages[0]

	if p.Name != "testpkg" {
		t.Errorf("RPM was named '%s' but expected testpkg", p.Name)
	}

	if len(p.Files) != 2 {
		// fatal since we access via indices below
		t.Fatalf("expected 2 files but got %d", len(p.Files))
	}

	if p.Files[0].Name != "f1" {
		t.Errorf("expected file name f1 but got %s", p.Files[0].Name)
	}

	if p.Files[1].Name != "f2" {
		t.Errorf("expected file name f2 but got %s", p.Files[1].Name)
	}
}

func TestStoreRPMInfoRedis(t *testing.T) {
	conn := redigomock.NewConn()
	cmds := []*redigomock.Cmd{
		conn.GenericCommand("SET").Expect("ok"),
		conn.GenericCommand("SADD").Expect("ok"),
		conn.GenericCommand("HMSET").Expect("ok"),
	}
	repo := &Repo{
		Name:    "testrepo",
		Version: "100",
		Type:    "B",
		Packages: []*RPM{
			{
				Name:     "testpkg",
				Version:  "100",
				Release:  "1",
				Provides: []string{"one", "two"},
				Files: []*File{
					{Name: "f1"},
					{Name: "f2"},
				},
			},
		},
	}

	if err := storeRepoInfoRedis(conn, repo); err != nil {
		t.Fatal(err)
	}

	for _, c := range cmds {
		if conn.Stats(c) == 0 {
			t.Errorf("expected command %s %s was not called", c.Name, c.Args)
		}
	}
}
