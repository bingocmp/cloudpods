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
	"net/http"
	"time"

	json "github.com/json-iterator/go"
	"yunion.io/x/jsonutils"
	api "yunion.io/x/onecloud/pkg/apis/generalservice"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/generalservice/models"
	"yunion.io/x/onecloud/pkg/generalservice/util"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
	"yunion.io/x/pkg/util/httputils"
	"yunion.io/x/pkg/util/printutils"
)

type InstanceSyncTask struct {
	baseTask
}

func init() {
	taskman.RegisterTask(InstanceSyncTask{})
}

func (its *InstanceSyncTask) taskProviderFailed(ctx context.Context, err error) {
	if its.ServiceProvider != nil {
		its.ServiceProvider.SetStatus(its.UserCred, api.INSTANCE_STATUS_SYNC_FAILED, err.Error())
	}
	its.SetStageFailed(ctx, jsonutils.NewString(err.Error()))
}

func (its *InstanceSyncTask) taskSucceed(ctx context.Context) {
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

func (its *InstanceSyncTask) taskFailed(ctx context.Context, err error) {
	if its.Instance != nil && its.TableSpec != nil {
		new := jsonutils.Marshal(its.Instance)
		new.(*jsonutils.JSONDict).Set("release_status", jsonutils.NewString(api.INSTANCE_STATUS_SYNC_FAILED))
		new.(*jsonutils.JSONDict).Set("release_fail_reason", jsonutils.NewString(err.Error()))

		its.TableSpec.Update(its.Instance, func() error {
			return models.UnmarshalSnakeCase([]byte(new.String()), &its.Instance)
		})
	}
}

func (its *InstanceSyncTask) taskComplete(ctx context.Context) {
	if its.ServiceProvider != nil {
		models.GeneralServiceProviderManager.TableSpec().Update(ctx, its.ServiceProvider, func() error {
			its.ServiceProvider.Status = api.INSTANCE_STATUS_SYNC_SUCCESS
			its.ServiceProvider.LastDeepSyncAt = time.Now()
			return nil
		})
	}
	its.SetStageComplete(ctx, nil)
}

func (its *InstanceSyncTask) OnInit(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	if err := its.baseTask.OnInit(ctx, obj, data); err != nil {
		its.taskFailed(ctx, err)
		return
	}

	client := &http.Client{}
	_, result, err := httputils.JSONRequest(client, ctx, httputils.GET, util.JoinUrl(its.Service.ApiEndpoint, "list"), its.getHeader(), nil, true)
	if err != nil {
		its.taskProviderFailed(ctx, err)
		return
	}

	items, err := result.GetArray()
	if err != nil {
		its.taskProviderFailed(ctx, err)
		return
	}

	dbItems, err := its.Service.GetDetailsList(ctx, its.UserCred, data)
	if err != nil {
		its.taskProviderFailed(ctx, err)
		return
	}

	var dbItemsTmp []map[string]interface{}
	dbItemsBytes, err := json.Marshal(dbItems.(printutils.ListResult).Data)
	if err != nil {
		its.taskProviderFailed(ctx, err)
		return
	}
	err = json.Unmarshal(dbItemsBytes, &dbItemsTmp)
	if err != nil {
		its.taskProviderFailed(ctx, err)
		return
	}

	primaryKey := stringutils2.Camel2Case(its.Service.PrimaryKey)
	for _, dbItem := range dbItemsTmp {
		exist := false
		for _, item := range items {
			primaryVal, _ := item.GetString(its.Service.PrimaryKey)
			if dbItem["external_id"] == primaryVal {
				exist = true
				break
			}
		}
		if !exist {
			err = its.TableSpec.DeleteFrom(map[string]interface{}{primaryKey: dbItem[primaryKey]})
			if err != nil {
				its.taskProviderFailed(ctx, err)
				return
			}
		}
	}

	for _, item := range items {
		primaryVal, _ := item.GetString(its.Service.PrimaryKey)
		item.(*jsonutils.JSONDict).Remove("id")

		err = models.UnmarshalSnakeCase([]byte(item.String()), &its.Instance)
		if err != nil {
			its.taskFailed(ctx, err)
			return
		}

		params := jsonutils.NewDict()
		params.Set(primaryKey, jsonutils.NewString(primaryVal))

		_, data, err := httputils.JSONRequest(client, ctx, httputils.GET, util.JoinUrl(its.Service.ApiEndpoint, "detail"), its.getHeader(), params, true)
		if err != nil {
			its.taskFailed(ctx, err)
			return
		}
		var detailTmp map[string]interface{}
		err = models.UnmarshalSnakeCase([]byte(data.String()), &detailTmp)
		if err != nil {
			its.taskFailed(ctx, err)
			return
		}

		item.(*jsonutils.JSONDict).Set("instance_detail", jsonutils.NewString(jsonutils.Marshal(detailTmp).String()))
		item.(*jsonutils.JSONDict).Set("provider_id", jsonutils.NewString(its.ServiceProvider.Id))

		if its.TableSpec.Query().Equals("external_id", primaryVal).Count() == 0 {
			item.(*jsonutils.JSONDict).Set("external_id", jsonutils.NewString(primaryVal))
			item.(*jsonutils.JSONDict).Set("id", jsonutils.NewString(db.DefaultUUIDGenerator()))
			err = models.UnmarshalSnakeCase([]byte(item.String()), &its.Instance)
			if err != nil {
				its.taskFailed(ctx, err)
				return
			}
			err = its.TableSpec.Insert(its.Instance)
			if err != nil {
				its.taskFailed(ctx, err)
				return
			}
		} else {
			err = its.TableSpec.Query().Equals("external_id", primaryVal).First(its.Instance)
			if err != nil {
				its.taskProviderFailed(ctx, err)
				return
			}
			_, err = its.TableSpec.Update(its.Instance, func() error {
				err = models.UnmarshalSnakeCase([]byte(item.String()), &its.Instance)
				return err
			})
			if err != nil {
				its.taskFailed(ctx, err)
				return
			}
		}
		its.taskSucceed(ctx)
	}

	its.taskComplete(ctx)
}
