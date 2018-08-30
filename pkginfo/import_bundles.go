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
	"github.com/clearlinux/diva/bundle"
	"github.com/gomodule/redigo/redis"
)

// ImportBundleDefinitions gets all of the bundle definitions and imports them
// into the database
func ImportBundleDefinitions(bundleInfo *BundleInfo) error {
	bundleDefinitions, err := bundle.GetAll(bundleInfo.BundleCache)
	if err != nil {
		return err
	}

	var c redis.Conn
	if c, err = initRedis(0); err != nil {
		return err
	}
	defer func() {
		_ = c.Close()
	}()

	err = storeBundleInfoRedis(c, bundleInfo, &bundleDefinitions)
	if err != nil {
		return err
	}

	return nil
}
