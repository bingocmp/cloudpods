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

	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/httperrors"
)

type SGeneralServiceLockManager struct {
	db.SStandaloneResourceBaseManager
}

var GeneralServiceLockManager *SGeneralServiceLockManager

func init() {
	GeneralServiceLockManager = &SGeneralServiceLockManager{
		SStandaloneResourceBaseManager: db.NewStandaloneResourceBaseManager(
			SGeneralServiceLock{},
			"generalservicelocks_tbl",
			"generalservicelock",
			"generalservicelocks",
		),
	}
	GeneralServiceLockManager.SetVirtualObject(GeneralServiceLockManager)
}

type SGeneralServiceLock struct {
	db.SStandaloneResourceBase
	ServiceId  string
	ProviderId string
	InstanceId string
}

func (its *SGeneralServiceLock) Keyword() string {
	return "generalservicelock"
}

func (its *SGeneralServiceLock) GetId() string {
	return its.Id
}

func (its *SGeneralServiceLockManager) GetLock(serviceId, providerId, instanceId string) *SGeneralServiceLock {
	lock := &SGeneralServiceLock{}
	its.Query().Equals("service_id", serviceId).Equals("provider_id", providerId).Equals("instance_id", instanceId).First(lock)
	if lock.Id != "" {
		return lock
	}
	lock.Id = db.DefaultUUIDGenerator()
	lock.ServiceId = serviceId
	lock.ProviderId = providerId
	lock.InstanceId = instanceId
	its.TableSpec().Insert(context.Background(), lock)
	return lock
}

func (its *SGeneralServiceLock) checkIsActive() (bool, error) {
	var openTask = true
	count, err := taskman.TaskManager.QueryTasksOfObject(its, time.Now().Add(-3*time.Minute), &openTask).CountWithError()
	if err != nil {
		return true, err
	}
	if count > 0 {
		return true, httperrors.NewBadRequestError("Instance has %d task active, can't run new tasks", count)
	}
	return false, nil
}
