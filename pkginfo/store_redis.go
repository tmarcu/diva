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
	"reflect"

	"github.com/clearlinux/diva/bundle"
	"github.com/clearlinux/diva/internal/helpers"
	"github.com/gomodule/redigo/redis"
)

func storeIterableRedisSet(c redis.Conn, key string, value []string) error {
	for i := range value {
		if err := c.Send("SADD", key, value[i]); err != nil {
			return err
		}
	}
	return c.Flush()
}

// storeRepoInfoRedis stores all data in repo to the running redis-server
func storeRepoInfoRedis(c redis.Conn, repo *Repo) error {
	repoKey := fmt.Sprintf("%s%s%s", repo.Name, repo.Version, repo.Type)
	if _, err := c.Do("SET", repoKey, repo.URI); err != nil {
		return err
	}

	for i := range repo.Packages {
		if err := storeRPMInfoRedis(c, repo, repo.Packages[i]); err != nil {
			return err
		}
	}

	return nil
}

// storeRPMInfoRedis stores the rpm under the constructed repo key in the
// running redis-server
func storeRPMInfoRedis(c redis.Conn, repo *Repo, rpm *RPM) error {
	repoKey := fmt.Sprintf("%s%s%s", repo.Name, repo.Version, repo.Type)
	if _, err := c.Do("SADD", repoKey+":packages", rpm.Name); err != nil {
		return err
	}
	pkgKey := fmt.Sprintf("%s:%s", repoKey, rpm.Name)
	_, err := c.Do("HMSET", redis.Args{}.Add(pkgKey).AddFlat(rpm)...)
	if err != nil {
		return err
	}

	// store file index mapping at reponame:packagename:files
	//             filename -> fileN
	// store each file map at reponame:packagename:fileN
	//             fileN -> File{}
	fKey := pkgKey + ":files"
	for fIdx, f := range rpm.Files {
		val := fmt.Sprintf("file%d", fIdx)
		fMap := map[string]string{f.Name: val}
		_, err := c.Do("HMSET", redis.Args{}.Add(fKey).AddFlat(fMap)...)
		if err != nil {
			return err
		}
		fIdxKey := fmt.Sprintf("%s:file%d", pkgKey, fIdx)
		_, err = c.Do("HMSET", redis.Args{}.Add(fIdxKey).AddFlat(f)...)
		if err != nil {
			return err
		}
	}
	return nil
}

func storeMapAsSliceRedis(c redis.Conn, key string, val map[string]bool) error {
	valSlice, err := helpers.HashmapToSortedSlice(val)
	if err != nil {
		return err
	}
	return storeIterableRedisSet(c, key, valSlice)
}

func storeBundleInfoRedis(c redis.Conn, bundleInfo *BundleInfo, bundleset *bundle.DefinitionsSet) error {
	bundlesKey := fmt.Sprintf("%s%sbundles", bundleInfo.Name, bundleInfo.Version)

	// convert bundle definition set to slice for flat data store
	bundles := bundle.SetToSlice(*bundleset)

	// store list of all bundles
	for _, bundle := range bundles {
		_, err := c.Do("SADD", bundlesKey, bundle.Name)
		if err != nil {
			return err
		}

		// store bundle definitions
		definitionKey := fmt.Sprintf("%s:%s", bundlesKey, bundle.Name)
		_, err = c.Do("HMSET", redis.Args{}.Add(definitionKey).AddFlat(bundle)...)
		if err != nil {
			return err
		}

		// store header information for each bundle
		header := reflect.ValueOf(&bundle.Header).Elem()
		for i := 0; i < header.NumField(); i++ {
			headerKey := header.Type().Field(i).Name
			headerValue := header.Field(i).Interface()
			if err = c.Send("SET", definitionKey+":"+headerKey, headerValue); err != nil {
				return err
			}
			if err = c.Flush(); err != nil {
				return err
			}
		}

		if err = storeMapAsSliceRedis(c, definitionKey+":includes", bundle.Includes); err != nil {
			return err
		}
		if err = storeMapAsSliceRedis(c, definitionKey+":directPackages", bundle.DirectPackages); err != nil {
			return err
		}
		if err = storeMapAsSliceRedis(c, definitionKey+":allPackages", bundle.AllPackages); err != nil {
			return err
		}
	}
	return nil
}
