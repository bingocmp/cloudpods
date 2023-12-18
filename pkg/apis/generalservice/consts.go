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

package generalservice

import "yunion.io/x/onecloud/pkg/apis"

const (
	SERVICE_TYPE = apis.SERVICE_TYPE_GENERALSERVICE

	INSTANCE_STATUS_DEPLOY_FAILED = "deploy_failed"

	INSTANCE_STATUS_DEPLOYING = "deploying"

	INSTANCE_STATUS_SYNC_FAILED = "sync_failed"

	INSTANCE_STATUS_SYNC_SUCCESS = "sync_success"

	INSTANCE_STATUS_SYNCING = "syncing"

	INSTANCE_STATUS_READY = "ready"

	SERVICE_UNAVAILABLE = "unavailable"

	SERVICE_AVAILABLE = "available"
)
