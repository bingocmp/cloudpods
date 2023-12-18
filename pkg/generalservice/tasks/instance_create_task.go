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
	"errors"
	"net/http"

	"yunion.io/x/jsonutils"
	api "yunion.io/x/onecloud/pkg/apis/generalservice"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/generalservice/models"
	"yunion.io/x/pkg/util/httputils"
)

type InstanceCreateTask struct {
	baseTask
}

func init() {
	taskman.RegisterTask(InstanceCreateTask{})
}

func (its *InstanceCreateTask) taskFailed(ctx context.Context, err error) {
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

func (its *InstanceCreateTask) taskComplete(ctx context.Context) {
	if its.Instance != nil && its.TableSpec != nil {
		new := jsonutils.Marshal(its.Instance)
		new.(*jsonutils.JSONDict).Set("release_status", jsonutils.NewString(api.INSTANCE_STATUS_READY))
		new.(*jsonutils.JSONDict).Set("release_fail_reason", jsonutils.NewString(""))

		its.TableSpec.Update(its.Instance, func() error {
			return models.UnmarshalSnakeCase([]byte(new.String()), &its.Instance)
		})
	}
	its.SetStageComplete(ctx, nil)
}

func (its *InstanceCreateTask) OnInit(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	if err := its.baseTask.OnInit(ctx, obj, data); err != nil {
		its.taskFailed(ctx, err)
		return
	}

	client := &http.Client{}
	_, result, err := httputils.JSONRequest(client, ctx, httputils.POST, its.Service.ApiEndpoint, its.getHeader(), data, true)
	if err != nil {
		its.taskFailed(ctx, err)
		return
	}

	_, err = its.TableSpec.Update(its.Instance, func() error {
		primaryVal, _ := result.GetString(its.Service.PrimaryKey)
		if primaryVal == "" {
			return errors.New("result must return `" + its.Service.PrimaryKey + "` value")
		}
		result.(*jsonutils.JSONDict).Set("external_id", jsonutils.NewString(primaryVal))
		return models.UnmarshalSnakeCase([]byte(result.String()), &its.Instance)
	})
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
