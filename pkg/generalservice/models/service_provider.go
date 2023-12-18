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

package models

import (
	"context"
	"time"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	apis "yunion.io/x/onecloud/pkg/apis/generalservice"
	"yunion.io/x/pkg/util/rbacscope"
	"yunion.io/x/sqlchemy"

	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/mcclient"
)

type SGeneralServiceProviderManager struct {
	db.SEnabledStatusStandaloneResourceBaseManager
}

var GeneralServiceProviderManager *SGeneralServiceProviderManager

func init() {
	GeneralServiceProviderManager = &SGeneralServiceProviderManager{
		SEnabledStatusStandaloneResourceBaseManager: db.NewEnabledStatusStandaloneResourceBaseManager(
			SGeneralServiceProvider{},
			"generalserviceproviders_tbl",
			"generalserviceprovider",
			"generalserviceproviders",
		),
	}
	GeneralServiceProviderManager.SetVirtualObject(GeneralServiceProviderManager)
}

type SGeneralServiceProvider struct {
	db.SEnabledStatusStandaloneResourceBase

	ServiceId string `width:"128" list:"user" update:"user" json:"service_id"`

	CloudRegionId string `width:"128" list:"user" update:"user" json:"cloud_region_id"`

	CloudRegion string `width:"128" list:"user" update:"user" json:"cloud_region"`

	ConsoleEndpoint string `nullable:"false" list:"user" update:"user" create:"required" json:"console_endpoint"`

	ProviderConfig string `length:"text" list:"user" update:"user" json:"provider_config"`

	LastDeepSyncAt time.Time `list:"user" update:"user" json:"last_deep_sync_at"`
}

func (its *SGeneralServiceProvider) IsSharable(reqCred mcclient.IIdentityProvider) bool {
	return true
}

func (its *SGeneralServiceProvider) GetOwnerId() mcclient.IIdentityProvider {
	owner := db.SOwnerId{}
	return &owner
}

func (its *SGeneralServiceProvider) PostUpdate(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) {
	//通知Websocket推送配置
	service := GeneralServiceManager.FetchServiceById(its.ServiceId)
	UpdatedChan <- service
}

func (its *SGeneralServiceProviderManager) ResourceScope() rbacscope.TRbacScope {
	return rbacscope.ScopeProject
}

func (its *SGeneralServiceProviderManager) ListItemFilter(ctx context.Context, q *sqlchemy.SQuery, userCred mcclient.TokenCredential, input apis.GeneralServiceListInput) (*sqlchemy.SQuery, error) {
	if input.ServiceId != "" {
		q = q.Equals("service_id", input.ServiceId)
	}
	if input.CloudRegionId != "" {
		q = q.Equals("cloud_region_id", input.CloudRegionId)
	}
	return q, nil
}

func (its *SGeneralServiceProviderManager) AllowDuplicateName() bool {
	return true
}

func (its *SGeneralServiceProviderManager) GetProviders(serviceId, regionId string) []SGeneralServiceProvider {
	q := its.Query()
	if serviceId != "" {
		q = q.Equals("service_id", serviceId)
	}
	if regionId != "" {
		q = q.Equals("cloud_region_id", regionId)
	}

	providers := make([]SGeneralServiceProvider, 0)
	err := db.FetchModelObjects(GeneralServiceProviderManager, q, &providers)
	if err != nil {
		log.Errorf("fetch general service providers error %s", err)
		return nil
	}
	return providers
}
