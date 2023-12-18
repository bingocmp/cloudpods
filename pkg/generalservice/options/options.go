// Copyright 2019 Yunion
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

package options

import (
	common_options "yunion.io/x/onecloud/pkg/cloudcommon/options"
)

type SGeneralServiceOptions struct {
	common_options.CommonOptions

	common_options.DBOptions
}

var (
	Options SGeneralServiceOptions
)

func OnOptionsChange(oldOptions, newOptions interface{}) bool {
	oldOpts := oldOptions.(*SGeneralServiceOptions)
	newOpts := newOptions.(*SGeneralServiceOptions)

	changed := false
	if common_options.OnBaseOptionsChange(&oldOpts.BaseOptions, &newOpts.BaseOptions) {
		changed = true
	}

	if common_options.OnDBOptionsChange(&oldOpts.DBOptions, &newOpts.DBOptions) {
		changed = true
	}

	return changed
}
