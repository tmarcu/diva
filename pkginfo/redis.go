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
	"strings"

	"github.com/gomodule/redigo/redis"
)

// initRedis checks whether a redis-server is running and initializes it if
// needed.
// TODO: actually start the redis server if not running, right now it just
// returns an error if there isn't one running.
func initRedis(port int) (redis.Conn, error) {
	p := ":6379"
	if port != 0 {
		p = fmt.Sprintf(":%d", port)
	}
	return redis.Dial("tcp", p)
}

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

	// store the requires
	rKey := pkgKey + ":requires"
	if err := storeIterableRedisSet(c, rKey, rpm.Requires); err != nil {
		return err
	}

	// store the provides
	pKey := pkgKey + ":provides"
	if err := storeIterableRedisSet(c, pKey, rpm.Provides); err != nil {
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

func getFilesRedis(c redis.Conn, repo *Repo, p *RPM) ([]*File, error) {
	pkgKey := fmt.Sprintf("%s%s%s:%s", repo.Name, repo.Version, repo.Type, p.Name)
	fIdxsKey := fmt.Sprintf("%s:files", pkgKey)
	fIdxs, err := redis.Strings(c.Do("HVALS", fIdxsKey))
	if err != nil {
		return []*File{}, err
	}

	files := []*File{}
	for _, fIdx := range fIdxs {
		fKey := fmt.Sprintf("%s:%s", pkgKey, fIdx)
		v, err := redis.Values(c.Do("HGETALL", fKey))
		if err != nil {
			return []*File{}, err
		}

		file := &File{}
		if err = redis.ScanStruct(v, file); err != nil {
			return []*File{}, err
		}

		files = append(files, file)
	}

	return files, nil
}

func getRPMRedis(c redis.Conn, repo *Repo, rpmName string) (*RPM, error) {
	var err error
	p := &RPM{}
	pkgKey := fmt.Sprintf("%s%s%s:%s", repo.Name, repo.Version, repo.Type, rpmName)
	p.Name, err = redis.String(c.Do("HGET", pkgKey, "Name"))
	if err != nil {
		return nil, err
	}

	p.Version, err = redis.String(c.Do("HGET", pkgKey, "Version"))
	if err != nil {
		return nil, err
	}

	p.Release, err = redis.String(c.Do("HGET", pkgKey, "Release"))
	if err != nil {
		return nil, err
	}

	p.Architecture, err = redis.String(c.Do("HGET", pkgKey, "Architecture"))
	if err != nil {
		return nil, err
	}

	p.SRPMName, err = redis.String(c.Do("HGET", pkgKey, "SRPMName"))
	if err != nil {
		return nil, err
	}

	p.License, err = redis.String(c.Do("HGET", pkgKey, "License"))
	if err != nil {
		return nil, err
	}

	rb, err := redis.Bytes(c.Do("HGET", pkgKey, "Requires"))
	if err != nil {
		return nil, err
	}
	p.Requires = strings.Fields(strings.Trim(string(rb), "[]"))

	bb, err := redis.Bytes(c.Do("HGET", pkgKey, "BuildRequires"))
	if err != nil {
		return nil, err
	}
	p.BuildRequires = strings.Fields(strings.Trim(string(bb), "[]"))

	pb, err := redis.Bytes(c.Do("HGET", pkgKey, "Provides"))
	if err != nil {
		return nil, err
	}
	p.Provides = strings.Fields(strings.Trim(string(pb), "[]"))

	p.Files, err = getFilesRedis(c, repo, p)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// getRepoRedis retrieves all data associated with the given repo from the
// running redis-server
func getRepoRedis(c redis.Conn, repo *Repo) error {
	repoKey := fmt.Sprintf("%s%s%s", repo.Name, repo.Version, repo.Type)
	pkgsKey := fmt.Sprintf("%s:packages", repoKey)
	pIdxs, err := redis.Strings(c.Do("SMEMBERS", pkgsKey))
	if err != nil {
		return err
	}
	for _, pn := range pIdxs {
		p, err := getRPMRedis(c, repo, pn)
		if err != nil {
			return err
		}
		repo.Packages = appendUniqueRPMName(repo.Packages, p)
	}

	return nil
}
