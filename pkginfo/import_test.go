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
)

func TestAppendUniqueRPMName(t *testing.T) {
	rpms := []*RPM{
		{Name: "0"},
		{Name: "1"},
		{Name: "2"},
		{Name: "3"},
	}

	rpmsToAppend := []*RPM{
		{Name: "1"},
		{Name: "4"},
	}

	for _, r := range rpmsToAppend {
		rpms = appendUniqueRPMName(rpms, r)
	}

	if len(rpms) != 5 {
		t.Errorf("expected 5 total RPMs but got %d", len(rpms))
	}

	for i, r := range rpms {
		if r.Name != fmt.Sprint(i) {
			t.Errorf("expected %s but got %s", fmt.Sprint(i), r.Name)
		}
	}
}
