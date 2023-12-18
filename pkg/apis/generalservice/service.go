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

type GeneralServiceListInput struct {
	CloudRegionId string `json:"cloud_region_id"`
	// swagger:ignore
	CloudRegion string `json:"cloud_region" yunion-deprecated-by:"cloud_region_id"`
	// swagger:ignore
	Region string `json:"region" yunion-deprecated-by:"cloud_region_id"`
	// swagger:ignore
	RegionId string `json:"region_id" yunion-deprecated-by:"cloud_region_id"`

	ServiceId string `json:"service_id"`
	// swagger:ignore
	Service string `json:"service" yunion-deprecated-by:"service_id"`

	// 列表排序时，用于排序的字段的名称，该字段不提供时，则按默认字段排序。一般时按照资源的新建时间逆序排序。
	OrderBy []string `json:"order_by"`
	// 列表排序时的顺序，desc为从高到低，asc为从低到高。默认是按照资源的创建时间desc排序。
	// example: desc|asc
	Order string `json:"order"`
}

type GeneralServiceDetails struct {
	SGeneralService

	Providers []SGeneralServiceProvider
}
