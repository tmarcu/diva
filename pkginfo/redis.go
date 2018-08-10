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
