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

	"yunion.io/x/jsonutils"
	api "yunion.io/x/onecloud/pkg/apis/generalservice"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/generalservice/models"
)

type InstanceOperateTask struct {
	baseTask
}

func init() {
	taskman.RegisterTask(InstanceOperateTask{})
}

func (its *InstanceOperateTask) taskFailed(ctx context.Context, err error) {
	if its.Instance != nil && its.TableSpec != nil {
		new := jsonutils.Marshal(its.Instance)
		new.(*jsonutils.JSONDict).Set("release_status", jsonutils.NewString(api.INSTANCE_STATUS_DEPLOY_FAILED))
		new.(*jsonutils.JSONDict).Set("release_fail_reason", jsonutils.NewString(err.Error()))

		its.TableSpec.Update(its.Instance, func() error {
			return models.UnmarshalSnakeCase([]byte(new.String()), &its.Instance)
		})
	}
	its.SetStageFailed(ctx, jsonutils.NewString(err.Error()))
}

func (its *InstanceOperateTask) taskComplete(ctx context.Context) {
	if its.Instance != nil && its.TableSpec != nil {
		new := jsonutils.Marshal(its.Instance)
		new.(*jsonutils.JSONDict).Set("release_fail_reason", jsonutils.NewString(""))

		its.TableSpec.Update(its.Instance, func() error {
			return models.UnmarshalSnakeCase([]byte(new.String()), &its.Instance)
		})
	}
	its.SetStageComplete(ctx, nil)
}

func (its *InstanceOperateTask) OnInit(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	if err := its.baseTask.OnInit(ctx, obj, data); err != nil {
		its.taskFailed(ctx, err)
		return
	}

	id, _ := jsonutils.Marshal(its.Instance).GetString("id")
	action, _ := data.GetString("__action")
	data.(*jsonutils.JSONDict).Remove("__action")

	_, err := its.Service.DoOperation(ctx, id, action, its.UserCred, data)
	if err != nil {
		its.taskFailed(ctx, err)
		return
	}

	subtask, err := taskman.TaskManager.NewTask(ctx, "InstanceSyncStatusTask", obj, its.GetUserCred(), its.GetParams(), its.GetTaskId(), "", nil)
	if err != nil {
		its.taskFailed(ctx, err)
		return
	}
	data.(*jsonutils.JSONDict).Set("needEvaluate", jsonutils.NewBool(true))
	subtask.ScheduleRun(data)

	its.taskComplete(ctx)
}
