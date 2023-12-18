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

package tasks

import (
	"context"
	"time"

	"yunion.io/x/jsonutils"
	api "yunion.io/x/onecloud/pkg/apis/generalservice"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/generalservice/models"
	"yunion.io/x/onecloud/pkg/generalservice/util"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
	"yunion.io/x/sqlchemy"
)

type InstanceSyncStatusTask struct {
	baseTask
}

func init() {
	taskman.RegisterTask(InstanceSyncStatusTask{})
}

func (its *InstanceSyncStatusTask) taskFailed(ctx context.Context, err error) {
	if its.Instance != nil && its.TableSpec != nil {
		newInstance := jsonutils.Marshal(its.Instance)
		newInstance.(*jsonutils.JSONDict).Set("release_status", jsonutils.NewString(api.INSTANCE_STATUS_SYNC_FAILED))
		newInstance.(*jsonutils.JSONDict).Set("release_fail_reason", jsonutils.NewString(err.Error()))

		its.TableSpec.Update(its.Instance, func() error {
			return models.UnmarshalSnakeCase([]byte(newInstance.String()), &its.Instance)
		})
	}
	its.SetStageFailed(ctx, jsonutils.NewString(err.Error()))
}

func (its *InstanceSyncStatusTask) taskComplete(ctx context.Context) {
	if its.Instance != nil && its.TableSpec != nil {
		newInstance := jsonutils.Marshal(its.Instance)
		newInstance.(*jsonutils.JSONDict).Set("release_fail_reason", jsonutils.NewString(""))

		tabSpec := sqlchemy.NewTableSpecFromStruct(its.Instance, stringutils2.Camel2Case(its.Service.Code)+"_tbl")
		tabSpec.Update(its.Instance, func() error {
			return models.UnmarshalSnakeCase([]byte(newInstance.String()), &its.Instance)
		})
	}
	its.SetStageComplete(ctx, nil)
}

func (its *InstanceSyncStatusTask) OnInit(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	if err := its.baseTask.OnInit(ctx, obj, data); err != nil {
		its.taskFailed(ctx, err)
		return
	}

	needEvaluate, _ := data.Bool("needEvaluate")
	id, _ := jsonutils.Marshal(its.Instance).GetString("id")
	externalId, _ := jsonutils.Marshal(its.Instance).GetString("external_id")
	data.(*jsonutils.JSONDict).Set(its.Service.PrimaryKey, jsonutils.NewString(externalId))

	var err error
	err = util.Wait(time.Second*5, time.Minute*5, func() (bool, error) {
		its.Instance, err = its.Service.SyncStatus(ctx, id, its.UserCred, data)
		if err != nil {
			return false, err
		}
		if needEvaluate {
			return its.Service.Evaluate(its.Instance)
		}
		return true, nil
	})
	if err != nil {
		its.taskFailed(ctx, err)
		return
	}

	its.taskComplete(ctx)
}
