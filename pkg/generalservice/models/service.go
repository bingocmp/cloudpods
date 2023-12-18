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
	"net/http"
	"strings"

	json "github.com/json-iterator/go"
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	api "yunion.io/x/onecloud/pkg/apis/generalservice"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/generalservice/util"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/mcclient/modules/identity"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/util/filterclause"
	"yunion.io/x/pkg/util/httputils"
	"yunion.io/x/pkg/util/rbacscope"
	"yunion.io/x/sqlchemy"

	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/util/stringutils2"

	"github.com/Knetic/govaluate"
	"github.com/json-iterator/go/extra"
)

var UpdatedChan = make(chan *SGeneralService, 1)

type SGeneralServiceManager struct {
	db.SEnabledStatusStandaloneResourceBaseManager
}

var GeneralServiceManager *SGeneralServiceManager

func init() {
	extra.RegisterFuzzyDecoders()
	GeneralServiceManager = &SGeneralServiceManager{
		SEnabledStatusStandaloneResourceBaseManager: db.NewEnabledStatusStandaloneResourceBaseManager(
			SGeneralService{},
			"generalservices_tbl",
			"generalservice",
			"generalservices",
		),
	}
	GeneralServiceManager.SetVirtualObject(GeneralServiceManager)
}

type SGeneralService struct {
	db.SEnabledStatusStandaloneResourceBase

	Code string `nullable:"false" list:"user" update:"user" create:"optional" json:"code"`

	Reason string `length:"text" nullable:"false" list:"user" update:"user" json:"reason"`

	PrimaryKey string `width:"30" nullable:"false" list:"user" update:"user" create:"required" json:"primary_key"`

	ApiEndpoint string `nullable:"false" default:"" list:"user" update:"user" create:"required" json:"api_endpoint"`

	ServiceConfig string `length:"text" nullable:"false" list:"user" update:"user" create:"required" json:"service_config"`

	DataSchema string `length:"text" nullable:"false" list:"user" update:"user" create:"required" json:"data_schema"`

	OperationSchema string `length:"text" nullable:"false" list:"user" update:"user" create:"required" json:"operation_schema"`

	CompletedExpression string `length:"text" nullable:"false" list:"user" update:"user" create:"required" json:"completed_expression"`
}

func (its *SGeneralService) IsSharable(reqCred mcclient.IIdentityProvider) bool {
	return true
}

func (its *SGeneralService) GetOwnerId() mcclient.IIdentityProvider {
	owner := db.SOwnerId{}
	return &owner
}

func (its *SGeneralService) CustomizeCreate(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data jsonutils.JSONObject) error {
	its.Id = its.Code
	instance, err := its.NewServiceInstance()
	if err != nil {
		return err
	}
	tableSpec := sqlchemy.NewTableSpecFromStruct(instance, stringutils2.Camel2Case(its.Code)+"_tbl")
	return tableSpec.Sync()
}

func (its *SGeneralService) ValidateUpdateCondition(ctx context.Context) error {
	instance, err := its.NewServiceInstance()
	if err != nil {
		return err
	}
	tableSpec := sqlchemy.NewTableSpecFromStruct(instance, stringutils2.Camel2Case(its.Code)+"_tbl")
	return tableSpec.Sync()
}

func (its *SGeneralService) GetDetailsProviders(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	providers := GeneralServiceProviderManager.GetProviders(its.Id, "")
	return jsonutils.Marshal(providers), nil
}

func (its *SGeneralService) GetDetailsList(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject) (interface{}, error) {
	instance, err := its.NewServiceInstance()
	if err != nil {
		return nil, err
	}

	tableSpec := sqlchemy.NewTableSpecFromStruct(instance, stringutils2.Camel2Case(its.Code)+"_tbl")
	q := tableSpec.Query()

	scope, _ := query.GetString("scope")
	if !userCred.HasSystemAdminPrivilege() || scope != "system" {
		q = q.Equals("tenant_id", userCred.GetProjectId())
	}

	log.Infof("%s %s", scope, userCred.String())

	conds := make([]sqlchemy.ICondition, 0)
	filterAny, _ := query.Bool("filter_any")
	filters := jsonutils.GetQueryStringArray(query, "filter")
	if len(filters) > 0 {
		for _, f := range filters {
			fc := filterclause.ParseFilterClause(f)
			if fc != nil {
				cond := fc.QueryCondition(q)
				if cond != nil {
					conds = append(conds, cond)
				}
			}
		}
		if len(conds) > 0 {
			if filterAny {
				q = q.Filter(sqlchemy.OR(conds...))
			} else {
				q = q.Filter(sqlchemy.AND(conds...))
			}
		}
	}

	limit, _ := query.Int("limit")
	offset, _ := query.Int("offset")

	orderBy, _ := query.GetArray("order_by")
	order, _ := query.GetString("order")

	interfaces, _ := its.NewServiceInstances()

	result, err := util.Paging(q, limit, offset, orderBy, order, interfaces)

	for _, datum := range result.Data {
		datum = its.formatInstance(datum)
	}

	return result, err
}

func (its *SGeneralService) PerformSync(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONDict) (jsonutils.JSONObject, error) {
	providers := GeneralServiceProviderManager.GetProviders(its.Id, "")

	for _, provider := range providers {
		provider.SetStatus(userCred, api.INSTANCE_STATUS_SYNCING, "")

		lock := GeneralServiceLockManager.GetLock(its.Id, provider.Id, "")
		task, err := taskman.TaskManager.NewTask(ctx, "InstanceSyncTask", lock, userCred, nil, "", "", nil)
		if err != nil {
			return nil, err
		}
		task.ScheduleRun(query)
	}

	return &jsonutils.JSONDict{}, nil
}

func (its *SGeneralService) PerformGet(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	id, _ := data.GetString("id")
	instance, _, err := its.GetInstanceById(id)
	if err != nil {
		return nil, err
	}
	return its.formatInstance(instance), nil
}

func (its *SGeneralService) PerformCreate(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	instance, err := its.NewServiceInstance()
	if err != nil {
		return nil, err
	}

	var projectId, project, domainId, projectDomain, regionId, regionName string

	if projectId = util.GetValueByKeys[string](data, "project_id", "projectId"); projectId == "" {
		projectId = userCred.GetProjectId()
	}

	sProj, _ := identity.Projects.GetById(auth.GetAdminSession(ctx, ""), projectId, nil)
	if sProj != nil {
		project, _ = sProj.GetString("name")
		domainId, _ = sProj.GetString("domain_id")
		projectDomain, _ = sProj.GetString("project_domain")
	} else {
		project = userCred.GetProjectName()
		domainId = userCred.GetProjectDomainId()
		projectDomain = userCred.GetProjectDomain()
	}

	regionId = util.GetValueByKeys[string](data, "cloudRegionId", "cloud_region_id")
	regionName = util.GetValueByKeys[string](data, "cloudRegionName", "cloud_region_name")

	provider, err := its.GetServiceProvider(ctx, regionId)
	if err != nil {
		return nil, errors.Errorf("fetch service provider error", err)
	}

	id := db.DefaultUUIDGenerator()
	newData := jsonutils.DeepCopy(data).(*jsonutils.JSONDict)
	newData.Set("domain_id", jsonutils.NewString(domainId))
	newData.Set("tenant_id", jsonutils.NewString(projectId))
	newData.Set("provider_id", jsonutils.NewString(provider.Id))
	newData.Set("cloud_region_id", jsonutils.NewString(regionId))
	newData.Set("cloud_region", jsonutils.NewString(regionName))
	newData.Set("project", jsonutils.NewString(project))
	newData.Set("project_domain", jsonutils.NewString(projectDomain))
	newData.Set("id", jsonutils.NewString(id))
	newData.Set("request_params", jsonutils.NewString(data.String()))
	newData.Set("release_status", jsonutils.NewString(api.INSTANCE_STATUS_DEPLOYING))
	newData.Set("release_fail_reason", jsonutils.NewString(""))
	newData.Set("last_operation", jsonutils.NewString("create"))

	tableSpec := sqlchemy.NewTableSpecFromStruct(instance, stringutils2.Camel2Case(its.Code)+"_tbl")
	err = UnmarshalSnakeCase([]byte(newData.String()), &instance)
	if err != nil {
		return nil, err
	}
	err = tableSpec.Insert(instance)
	if err != nil {
		return nil, err
	}

	lock := GeneralServiceLockManager.GetLock(its.Id, provider.Id, id)
	task, err := taskman.TaskManager.NewTask(ctx, "InstanceCreateTask", lock, userCred, nil, "", "", nil)
	if err != nil {
		return nil, err
	}

	task.ScheduleRun(data)

	return jsonutils.Marshal(instance), err
}

func (its *SGeneralService) PerformUpdate(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	id, _ := data.GetString("id")
	instance, provider, err := its.GetInstanceById(id)
	if err != nil {
		return nil, err
	}

	externalId, _ := jsonutils.Marshal(instance).GetString("external_id")
	newData := jsonutils.DeepCopy(data).(*jsonutils.JSONDict)
	newData.Set(its.PrimaryKey, jsonutils.NewString(externalId))
	newData.Set("release_status", jsonutils.NewString(api.INSTANCE_STATUS_DEPLOYING))
	newData.Set("release_fail_reason", jsonutils.NewString(""))
	newData.Set("last_operation", jsonutils.NewString("update"))

	tableSpec := sqlchemy.NewTableSpecFromStruct(instance, stringutils2.Camel2Case(its.Code)+"_tbl")
	_, err = tableSpec.Update(instance, func() error {
		var allParams, newParams map[string]interface{}
		reqParams, _ := jsonutils.Marshal(instance).GetString("request_params")
		if reqParams != "" {
			json.Unmarshal([]byte(reqParams), &allParams)
		}
		json.Unmarshal([]byte(data.String()), &newParams)
		if allParams != nil && newParams != nil {
			util.Copy(allParams, newParams)
		}
		newData.Set("request_params", jsonutils.NewString(jsonutils.Marshal(allParams).String()))
		return UnmarshalSnakeCase([]byte(newData.String()), &instance)
	})
	if err != nil {
		return nil, err
	}

	lock := GeneralServiceLockManager.GetLock(its.Id, provider.Id, id)
	_, err = lock.checkIsActive()
	if err != nil {
		return nil, err
	}

	task, err := taskman.TaskManager.NewTask(ctx, "InstanceUpdateTask", lock, userCred, nil, "", "", nil)
	if err != nil {
		return nil, err
	}

	task.ScheduleRun(data)

	return jsonutils.Marshal(instance), err
}

func (its *SGeneralService) PerformSyncstatus(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	id, _ := data.GetString("id")
	sync, _ := query.Bool("sync")

	if sync {
		instance, err := its.SyncStatus(ctx, id, userCred, data)
		return its.formatInstance(instance), err
	}

	instance, provider, err := its.GetInstanceById(id)
	if err != nil {
		return nil, err
	}

	externalId, _ := jsonutils.Marshal(instance).GetString("external_id")
	tableSpec := sqlchemy.NewTableSpecFromStruct(instance, stringutils2.Camel2Case(its.Code)+"_tbl")

	newData := jsonutils.DeepCopy(data).(*jsonutils.JSONDict)
	newData.Set(its.PrimaryKey, jsonutils.NewString(externalId))
	newData.Set("release_status", jsonutils.NewString(api.INSTANCE_STATUS_SYNCING))
	newData.Set("release_fail_reason", jsonutils.NewString(""))

	_, err = tableSpec.Update(instance, func() error {
		return UnmarshalSnakeCase([]byte(newData.String()), &instance)
	})
	if err != nil {
		return nil, err
	}

	lock := GeneralServiceLockManager.GetLock(its.Id, provider.Id, id)
	_, err = lock.checkIsActive()
	if err != nil {
		return nil, err
	}

	task, err := taskman.TaskManager.NewTask(ctx, "InstanceSyncStatusTask", lock, userCred, nil, "", "", nil)
	if err != nil {
		return nil, err
	}

	task.ScheduleRun(data)

	return &jsonutils.JSONDict{}, nil
}

func (its *SGeneralService) PerformMetrics(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	id, _ := data.GetString("id")
	instance, provider, err := its.GetInstanceById(id)
	if err != nil {
		return nil, err
	}
	externalId, _ := jsonutils.Marshal(instance).GetString("external_id")
	data.(*jsonutils.JSONDict).Set(its.PrimaryKey, jsonutils.NewString(externalId))

	header := http.Header{
		"X-Auth-Token":  []string{userCred.GetTokenString()},
		"X-Provider-Id": []string{provider.Id},
	}

	client := &http.Client{}
	_, body, err := httputils.JSONRequest(client, ctx, httputils.GET, util.JoinUrl(its.ApiEndpoint, externalId, "metrics")+"?"+data.QueryString(), header, nil, true)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (its *SGeneralService) PerformOperate(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	action, _ := query.GetString("action")
	if action == "" {
		action, _ = data.GetString("action")
	}
	if action == "" {
		return nil, errors.Errorf("empty action")
	}

	id, _ := data.GetString("id")
	sync, _ := query.Bool("sync")

	if sync {
		return its.DoOperation(ctx, id, action, userCred, data)
	}

	providerId := ""
	if id != "" {
		instance, provider, _ := its.GetInstanceById(id)
		if instance != nil {
			externalId, _ := jsonutils.Marshal(instance).GetString("external_id")
			data.(*jsonutils.JSONDict).Set(its.PrimaryKey, jsonutils.NewString(externalId))
		}
		if provider != nil {
			providerId = provider.Id
		}
	}

	data.(*jsonutils.JSONDict).Set("__action", jsonutils.NewString(action))

	lock := GeneralServiceLockManager.GetLock(its.Id, providerId, id)
	_, err := lock.checkIsActive()
	if err != nil {
		return nil, err
	}

	task, err := taskman.TaskManager.NewTask(ctx, "InstanceOperateTask", lock, userCred, nil, "", "", nil)
	if err != nil {
		return nil, err
	}

	task.ScheduleRun(data)

	return &jsonutils.JSONDict{}, nil
}

func (its *SGeneralService) PerformDelete(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	id, _ := data.GetString("id")
	instance, provider, err := its.GetInstanceById(id)
	if err != nil {
		return nil, err
	}
	externalId, _ := jsonutils.Marshal(instance).GetString("external_id")
	data.(*jsonutils.JSONDict).Set(its.PrimaryKey, jsonutils.NewString(externalId))

	var result jsonutils.JSONObject

	if externalId != "" {
		header := http.Header{
			"X-Auth-Token":  []string{userCred.GetTokenString()},
			"X-Provider-Id": []string{provider.Id},
		}

		client := &http.Client{}
		_, result, err = httputils.JSONRequest(client, ctx, httputils.DELETE, util.JoinUrl(its.ApiEndpoint, externalId), header, data, true)
		if err != nil && !strings.Contains(err.Error(), "该实例不存在") {
			return nil, err
		}
	}

	tableSpec := sqlchemy.NewTableSpecFromStruct(instance, stringutils2.Camel2Case(its.Code)+"_tbl")
	tableSpec.DeleteFrom(map[string]interface{}{"id": id})

	return result, err
}

func (its *SGeneralService) PerformManager(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	uri, _ := data.GetString("uri")
	method, _ := data.GetString("method")
	params, _ := data.Get("params")

	if !strings.HasPrefix(uri, "/") {
		uri = "/" + uri
	}

	requestUrl := strings.TrimRight(its.ApiEndpoint, "/") + uri

	header := http.Header{
		"X-Auth-Token": []string{userCred.GetTokenString()},
	}

	client := &http.Client{}

	_, result, err := httputils.JSONRequest(client, ctx, httputils.THttpMethod(strings.ToUpper(method)), requestUrl, header, params, true)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (its *SGeneralService) GetInstanceById(id string) (interface{}, *SGeneralServiceProvider, error) {
	instance, err := its.NewServiceInstance()
	if err != nil {
		return nil, nil, err
	}
	tableSpec := sqlchemy.NewTableSpecFromStruct(instance, stringutils2.Camel2Case(its.Code)+"_tbl")

	err = tableSpec.Query().Equals("id", id).First(instance)
	if err != nil {
		return nil, nil, err
	}

	providerId, _ := jsonutils.Marshal(instance).GetString("provider_id")
	model, err := GeneralServiceProviderManager.FetchById(providerId)
	if err != nil {
		return nil, nil, errors.Wrap(err, "fetch GeneralServiceProvider")
	}
	provider := model.(*SGeneralServiceProvider)

	return instance, provider, err
}

func (its *SGeneralService) Evaluate(instance interface{}) (bool, error) {
	if its.CompletedExpression == "" {
		return true, nil
	}

	var params map[string]interface{}
	err := json.Unmarshal([]byte(jsonutils.Marshal(instance).String()), &params)
	if err != nil {
		return false, err
	}

	expression, err := govaluate.NewEvaluableExpression(its.CompletedExpression)
	if err != nil {
		return false, err
	}
	result, err := expression.Evaluate(params)
	if err != nil {
		return false, err
	}

	log.Infof("Evaluate service instance %s: %v", its.CompletedExpression, result)

	return result.(bool), nil
}

func (its *SGeneralService) SyncStatus(ctx context.Context, id string, userCred mcclient.TokenCredential, data jsonutils.JSONObject) (interface{}, error) {
	instance, provider, err := its.GetInstanceById(id)
	if err != nil {
		return nil, err
	}

	externalId, _ := jsonutils.Marshal(instance).GetString("external_id")

	data.(*jsonutils.JSONDict).Set(its.PrimaryKey, jsonutils.NewString(externalId))

	header := http.Header{
		"X-Auth-Token":  []string{userCred.GetTokenString()},
		"X-Provider-Id": []string{provider.Id},
	}

	client := &http.Client{}
	_, instanceResult, err := httputils.JSONRequest(client, ctx, httputils.GET, util.JoinUrl(its.ApiEndpoint, externalId), header, data, true)
	if err != nil {
		return nil, err
	}
	_, detailResult, err := httputils.JSONRequest(client, ctx, httputils.GET, util.JoinUrl(its.ApiEndpoint, externalId, "detail"), header, data, true)
	if err != nil {
		return nil, err
	}

	tableSpec := sqlchemy.NewTableSpecFromStruct(instance, stringutils2.Camel2Case(its.Code)+"_tbl")

	_, err = tableSpec.Update(instance, func() error {
		id, _ := jsonutils.Marshal(instance).GetString("id")
		domainId, _ := jsonutils.Marshal(instance).GetString("domain_id")
		tenantId, _ := jsonutils.Marshal(instance).GetString("tenant_id")

		err = UnmarshalSnakeCase([]byte(instanceResult.String()), &instance)
		if err != nil {
			return err
		}

		var detailTmp map[string]interface{}
		err = UnmarshalSnakeCase([]byte(detailResult.String()), &detailTmp)
		if err != nil {
			return err
		}

		newInstance := jsonutils.Marshal(instance)
		newInstance.(*jsonutils.JSONDict).Set("id", jsonutils.NewString(id))
		newInstance.(*jsonutils.JSONDict).Set("domain_id", jsonutils.NewString(domainId))
		newInstance.(*jsonutils.JSONDict).Set("tenant_id", jsonutils.NewString(tenantId))
		newInstance.(*jsonutils.JSONDict).Set("instance_detail", jsonutils.NewString(jsonutils.Marshal(detailTmp).String()))

		return UnmarshalSnakeCase([]byte(newInstance.String()), &instance)
	})

	return instance, nil
}

func (its *SGeneralService) GetServiceProvider(ctx context.Context, regionId string) (*SGeneralServiceProvider, error) {
	var providers []SGeneralServiceProvider
	providers = GeneralServiceProviderManager.GetProviders(its.Id, regionId)
	if len(providers) == 0 {
		providers = GeneralServiceProviderManager.GetProviders(its.Id, "")
	}
	if len(providers) == 0 {
		return nil, errors.Errorf("service `%v` does not define provider", its.Id)
	}
	return &providers[0], nil
}

func (its *SGeneralService) DoOperation(ctx context.Context, id, action string, userCred mcclient.TokenCredential, data jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	var externalId string
	var instance interface{}
	var provider *SGeneralServiceProvider
	header := http.Header{
		"X-Auth-Token": []string{userCred.GetTokenString()},
	}

	instance, provider, _ = its.GetInstanceById(id)
	if instance != nil {
		externalId, _ = jsonutils.Marshal(instance).GetString("external_id")
		data.(*jsonutils.JSONDict).Set(its.PrimaryKey, jsonutils.NewString(externalId))
	}
	if provider != nil {
		header.Add("X-Provider-Id", provider.Id)
	}

	client := &http.Client{}
	_, result, err := httputils.JSONRequest(client, ctx, httputils.POST, util.JoinUrl(its.ApiEndpoint, externalId, action), header, data, true)
	if err != nil {
		return nil, err
	}

	if instance != nil {
		tableSpec := sqlchemy.NewTableSpecFromStruct(instance, stringutils2.Camel2Case(its.Code)+"_tbl")

		newData := jsonutils.DeepCopy(data).(*jsonutils.JSONDict)
		newData.Set("last_operation", jsonutils.NewString(action))

		_, err = tableSpec.Update(instance, func() error {
			return UnmarshalSnakeCase([]byte(newData.String()), &instance)
		})
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (its *SGeneralService) formatInstance(instance interface{}) jsonutils.JSONObject {
	result := jsonutils.Marshal(instance)
	detailStr, _ := result.GetString("instance_detail")
	if detailStr != "" {
		detailObj, _ := jsonutils.ParseString(detailStr)
		result.(*jsonutils.JSONDict).Set("instance_detail", detailObj)
	}
	requestStr, _ := result.GetString("request_params")
	if requestStr != "" {
		requestObj, _ := jsonutils.ParseString(requestStr)
		result.(*jsonutils.JSONDict).Set("request_params", requestObj)
	}
	return result
}

func (its *SGeneralServiceManager) ValidateCreateData(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data *jsonutils.JSONDict) (*jsonutils.JSONDict, error) {
	dataSchema, err := data.GetString("data_schema")
	if err != nil {
		return nil, err
	}
	gs := &SGeneralService{DataSchema: dataSchema}
	instance, err := gs.NewServiceInstance()
	if instance == nil || err != nil {
		return nil, errors.Errorf("invalid data schema")
	}
	return data, nil
}

func (its *SGeneralServiceManager) ResourceScope() rbacscope.TRbacScope {
	return rbacscope.ScopeProject
}

func (its *SGeneralServiceManager) ListItemFilter(ctx context.Context, q *sqlchemy.SQuery, userCred mcclient.TokenCredential, query jsonutils.JSONObject) (*sqlchemy.SQuery, error) {
	return q, nil
}

func (its *SGeneralServiceManager) AllowDuplicateName() bool {
	return true
}

func (its *SGeneralServiceManager) InitializeData() error {
	var services []SGeneralService
	err := GeneralServiceManager.Query().All(&services)
	if err != nil {
		log.Errorf("fetch general service error %s", err)
		return nil
	}
	for _, service := range services {
		instance, err := service.NewServiceInstance()
		if err != nil {
			return err
		}
		tableSpec := sqlchemy.NewTableSpecFromStruct(instance, stringutils2.Camel2Case(service.Code)+"_tbl")
		tableSpec.Sync()
	}
	return nil
}

func (its *SGeneralServiceManager) FetchServiceById(id string) *SGeneralService {
	obj, err := its.FetchById(id)
	if err != nil {
		log.Errorf("general service %s %s", id, err)
		return nil
	}
	return obj.(*SGeneralService)
}

func (its *SGeneralServiceManager) GetResourceCount() ([]db.SScopeResourceCount, error) {
	var services []SGeneralService
	err := its.Query().All(&services)
	if err != nil {
		return nil, err
	}
	result := []db.SScopeResourceCount{{DomainId: "default", ResCount: len(services)}}
	for _, service := range services {
		instance, err := service.NewServiceInstance()
		if err != nil {
			return nil, err
		}
		tableSpec := sqlchemy.NewTableSpecFromStruct(instance, stringutils2.Camel2Case(service.Code)+"_tbl")
		children, err := db.CalculateResourceCount(tableSpec.Query(), "tenant_id")
		if err != nil {
			return nil, err
		}
		if len(children) == 0 {
			result[0].ChildResources = append(result[0].ChildResources, db.SScopeResourceCount{DomainId: "default", Resource: service.Code})
		} else {
			for i := range children {
				children[i].Resource = service.Code
			}
			result[0].ChildResources = append(result[0].ChildResources, children...)
		}
	}
	return result, nil
}
