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

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/tristate"
	"yunion.io/x/pkg/util/rbacscope"
	"yunion.io/x/sqlchemy"

	api "yunion.io/x/onecloud/pkg/apis/identity"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
)

// +onecloud:swagger-gen-ignore
type IIdentityModelManager interface {
	db.IStandaloneModelManager

	GetIIdentityModelManager() IIdentityModelManager
}

type IIdentityModel interface {
	db.IStandaloneModel

	GetIIdentityModelManager() IIdentityModelManager

	GetIIdentityModel() IIdentityModel
}

// +onecloud:swagger-gen-ignore
type SIdentityBaseResourceManager struct {
	db.SStandaloneResourceBaseManager
	db.SDomainizedResourceBaseManager
}

func NewIdentityBaseResourceManager(dt interface{}, tableName string, keyword string, keywordPlural string) SIdentityBaseResourceManager {
	return SIdentityBaseResourceManager{
		SStandaloneResourceBaseManager: db.NewStandaloneResourceBaseManager(dt, tableName, keyword, keywordPlural),
	}
}

type SIdentityBaseResource struct {
	db.SStandaloneResourceBase
	db.SDomainizedResourceBase

	// 额外信息
	Extra *jsonutils.JSONDict `nullable:"true"`
	// DomainId string `width:"64" charset:"ascii" default:"default" nullable:"false" index:"true" list:"user"`
}

// +onecloud:swagger-gen-ignore
type SEnabledIdentityBaseResourceManager struct {
	SIdentityBaseResourceManager
}

func NewEnabledIdentityBaseResourceManager(dt interface{}, tableName string, keyword string, keywordPlural string) SEnabledIdentityBaseResourceManager {
	return SEnabledIdentityBaseResourceManager{
		SIdentityBaseResourceManager: NewIdentityBaseResourceManager(dt, tableName, keyword, keywordPlural),
	}
}

type SEnabledIdentityBaseResource struct {
	SIdentityBaseResource

	Enabled tristate.TriState `default:"true" list:"user" update:"domain" create:"domain_optional"`
}

func (model *SIdentityBaseResource) GetIIdentityModelManager() IIdentityModelManager {
	return model.GetModelManager().(IIdentityModelManager)
}

func (model *SIdentityBaseResource) GetIIdentityModel() IIdentityModel {
	return model.GetVirtualObject().(IIdentityModel)
}

// func (model *SIdentityBaseResource) IsOwner(userCred mcclient.TokenCredential) bool {
// 	return userCred.GetProjectDomainId() == model.DomainId
// }

func (model *SIdentityBaseResource) GetDomain() *SDomain {
	if len(model.DomainId) > 0 && model.DomainId != api.KeystoneDomainRoot {
		domain, err := DomainManager.FetchDomainById(model.DomainId)
		if err != nil {
			log.Errorf("GetDomain fail %s", err)
		}
		return domain
	}
	return nil
}

func (manager *SIdentityBaseResourceManager) GetIIdentityModelManager() IIdentityModelManager {
	return manager.GetVirtualObject().(IIdentityModelManager)
}

func (manager *SIdentityBaseResourceManager) FetchByName(userCred mcclient.IIdentityProvider, idStr string) (db.IModel, error) {
	return db.FetchByName(manager.GetIIdentityModelManager(), userCred, idStr)
}

func (manager *SIdentityBaseResourceManager) FetchByIdOrName(userCred mcclient.IIdentityProvider, idStr string) (db.IModel, error) {
	return db.FetchByIdOrName(manager.GetIIdentityModelManager(), userCred, idStr)
}

func (manager *SIdentityBaseResourceManager) ListItemFilter(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.IdentityBaseResourceListInput,
) (*sqlchemy.SQuery, error) {
	var err error
	q, err = manager.SStandaloneResourceBaseManager.ListItemFilter(ctx, q, userCred, query.StandaloneResourceListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SStandaloneResourceBaseManager.ListItemFilter")
	}
	q, err = manager.SDomainizedResourceBaseManager.ListItemFilter(ctx, q, userCred, query.DomainizedResourceListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SDomainizedResourceBaseManager.ListItemFilter")
	}
	return q, nil
}

func (manager *SEnabledIdentityBaseResourceManager) ListItemFilter(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.EnabledIdentityBaseResourceListInput,
) (*sqlchemy.SQuery, error) {
	q, err := manager.SIdentityBaseResourceManager.ListItemFilter(ctx, q, userCred, query.IdentityBaseResourceListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SIdentityBaseResourceManager.ListItemFilter")
	}
	if query.Enabled != nil {
		if *query.Enabled {
			q = q.IsTrue("enabled")
		} else {
			q = q.IsFalse("enabled")
		}
	}
	return q, nil
}

func (manager *SIdentityBaseResourceManager) OrderByExtraFields(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.IdentityBaseResourceListInput,
) (*sqlchemy.SQuery, error) {
	q, err := manager.SStandaloneResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.StandaloneResourceListInput)
	if err != nil {
		return nil, err
	}
	orderByDomain := query.OrderByDomain
	if sqlchemy.SQL_ORDER_ASC.Equals(orderByDomain) || sqlchemy.SQL_ORDER_DESC.Equals(orderByDomain) {
		domains := DomainManager.Query().SubQuery()
		q = q.LeftJoin(domains, sqlchemy.Equals(q.Field("domain_id"), domains.Field("id")))
		if sqlchemy.SQL_ORDER_ASC.Equals(orderByDomain) {
			q = q.Asc(domains.Field("name"))
		} else {
			q = q.Desc(domains.Field("name"))
		}
	}
	return q, nil
}

func (manager *SEnabledIdentityBaseResourceManager) OrderByExtraFields(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.EnabledIdentityBaseResourceListInput,
) (*sqlchemy.SQuery, error) {
	var err error
	q, err = manager.SIdentityBaseResourceManager.OrderByExtraFields(ctx, q, userCred, query.IdentityBaseResourceListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SIdentityBaseResourceManager.OrderByExtraFields")
	}
	return q, nil
}

func (manager *SIdentityBaseResourceManager) QueryDistinctExtraField(q *sqlchemy.SQuery, field string) (*sqlchemy.SQuery, error) {
	var err error
	if field == "domain" {
		domainQuery := DomainManager.Query("name", "id").Distinct().SubQuery()
		q.AppendField(domainQuery.Field("name", "domain"))
		q = q.Join(domainQuery, sqlchemy.Equals(q.Field("domain_id"), domainQuery.Field("id")))
		q.GroupBy(domainQuery.Field("name"))
		return q, nil
	}
	q, err = manager.SStandaloneResourceBaseManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}
	return q, httperrors.ErrNotFound
}

func (manager *SEnabledIdentityBaseResourceManager) QueryDistinctExtraField(q *sqlchemy.SQuery, field string) (*sqlchemy.SQuery, error) {
	q, err := manager.SIdentityBaseResourceManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}
	return q, httperrors.ErrNotFound
}

/*func fetchDomainInfo(data jsonutils.JSONObject) (mcclient.IIdentityProvider, error) {
	domainId, key := jsonutils.GetAnyString2(data, []string{"domain_id", "project_domain", "project_domain_id"})
	if len(domainId) > 0 {
		data.(*jsonutils.JSONDict).Remove(key)
		domain, err := DomainManager.FetchDomainByIdOrName(domainId)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, httperrors.NewResourceNotFoundError2(DomainManager.Keyword(), domainId)
			}
			return nil, httperrors.NewGeneralError(err)
		}
		owner := db.SOwnerId{DomainId: domain.Id, Domain: domain.Name}
		data.(*jsonutils.JSONDict).Set("project_domain", jsonutils.NewString(domain.Id))
		return &owner, nil
	}
	return nil, nil
}

func (manager *SIdentityBaseResourceManager) FetchOwnerId(ctx context.Context, data jsonutils.JSONObject) (mcclient.IIdentityProvider, error) {
	return fetchDomainInfo(data)
}*/

func (manager *SIdentityBaseResourceManager) ValidateCreateData(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, input api.IdentityBaseResourceCreateInput) (api.IdentityBaseResourceCreateInput, error) {
	domain, _ := DomainManager.FetchDomainById(ownerId.GetProjectDomainId())
	if domain.Enabled.IsFalse() {
		return input, httperrors.NewInvalidStatusError("domain is disabled")
	}
	var err error
	input.StandaloneResourceCreateInput, err = manager.SStandaloneResourceBaseManager.ValidateCreateData(ctx, userCred, ownerId, query, input.StandaloneResourceCreateInput)
	if err != nil {
		return input, err
	}
	return input, nil
}

func (manager *SEnabledIdentityBaseResourceManager) ValidateCreateData(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, input api.EnabledIdentityBaseResourceCreateInput) (api.EnabledIdentityBaseResourceCreateInput, error) {
	var err error
	input.IdentityBaseResourceCreateInput, err = manager.SIdentityBaseResourceManager.ValidateCreateData(ctx, userCred, ownerId, query, input.IdentityBaseResourceCreateInput)
	if err != nil {
		return input, err
	}
	return input, nil
}

func (model *SIdentityBaseResource) ValidateUpdateData(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	input api.IdentityBaseUpdateInput,
) (api.IdentityBaseUpdateInput, error) {
	var err error
	input.StandaloneResourceBaseUpdateInput, err = model.SStandaloneResourceBase.ValidateUpdateData(ctx, userCred, query, input.StandaloneResourceBaseUpdateInput)
	if err != nil {
		return input, errors.Wrap(err, "SStandaloneResourceBase.ValidateUpdateData")
	}
	return input, nil
}

func (model *SEnabledIdentityBaseResource) ValidateUpdateData(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	input api.EnabledIdentityBaseUpdateInput,
) (api.EnabledIdentityBaseUpdateInput, error) {
	var err error
	input.IdentityBaseUpdateInput, err = model.SIdentityBaseResource.ValidateUpdateData(ctx, userCred, query, input.IdentityBaseUpdateInput)
	if err != nil {
		return input, errors.Wrap(err, "SIdentityBaseResource.ValidateUpdateData")
	}
	return input, nil
}

/*func(manager *SIdentityBaseResourceManager) NamespaceScope() rbacutils.TRbacScope {
	return rbacutils.ScopeSystem
}*/

func (manager *SIdentityBaseResourceManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []api.IdentityBaseResourceDetails {
	rows := make([]api.IdentityBaseResourceDetails, len(objs))

	stdRows := manager.SStandaloneResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)
	domainRows := manager.SDomainizedResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)

	// domainIds := stringutils2.SSortedStrings{}
	for i := range rows {
		rows[i] = api.IdentityBaseResourceDetails{
			StandaloneResourceDetails: stdRows[i],
			DomainizedResourceInfo:    domainRows[i],
		}
		/*var base *SIdentityBaseResource
		reflectutils.FindAnonymouStructPointer(objs[i], &base)
		if base != nil && len(base.DomainId) > 0 && base.DomainId != api.KeystoneDomainRoot {
			domainIds = stringutils2.Append(domainIds, base.DomainId)
		}*/
	}

	/*if len(fields) == 0 || fields.Contains("project_domain") {
		domains := fetchDomain(domainIds)
		if domains != nil {
			for i := range rows {
				var base *SIdentityBaseResource
				reflectutils.FindAnonymouStructPointer(objs[i], &base)
				if base != nil && len(base.DomainId) > 0 && base.DomainId != api.KeystoneDomainRoot {
					if domain, ok := domains[base.DomainId]; ok {
						rows[i].ProjectDomain = domain.Name
					}
				}
			}
		}
	}*/
	return rows
}

func (manager *SEnabledIdentityBaseResourceManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []api.EnabledIdentityBaseResourceDetails {
	rows := make([]api.EnabledIdentityBaseResourceDetails, len(objs))

	identRows := manager.SIdentityBaseResourceManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)

	for i := range rows {
		rows[i] = api.EnabledIdentityBaseResourceDetails{
			IdentityBaseResourceDetails: identRows[i],
		}
	}

	return rows
}

/*
func fetchDomain(domainIds []string) map[string]SDomain {
	q := DomainManager.Query().In("id", domainIds)
	domains := make([]SDomain, 0)
	err := db.FetchModelObjects(DomainManager, q, &domains)
	if err != nil {
		return nil
	}
	ret := make(map[string]SDomain)
	for i := range domains {
		ret[domains[i].Id] = domains[i]
	}
	return ret
}*/

func (model *SIdentityBaseResource) CustomizeCreate(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data jsonutils.JSONObject) error {
	model.DomainId = ownerId.GetProjectDomainId()
	return model.SStandaloneResourceBase.CustomizeCreate(ctx, userCred, ownerId, query, data)
}

/*
func (self *SIdentityBaseResource) ValidateDeleteCondition(ctx context.Context) error {
	return self.SStandaloneResourceBase.ValidateDeleteCondition(ctx)
}

func (self *SIdentityBaseResource) ValidateUpdateData(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data *jsonutils.JSONDict) (*jsonutils.JSONDict, error) {
	return self.SStandaloneResourceBase.ValidateUpdateData(ctx, userCred, query, data)
}
*/

func (self *SEnabledIdentityBaseResource) ValidateDeleteCondition(ctx context.Context, info jsonutils.JSONObject) error {
	if self.Enabled.IsTrue() {
		return httperrors.NewResourceBusyError("resource is enabled")
	}
	return self.SIdentityBaseResource.ValidateDeleteCondition(ctx, nil)
}

func (model *SIdentityBaseResource) PostCreate(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data jsonutils.JSONObject) {
	model.SStandaloneResourceBase.PostCreate(ctx, userCred, ownerId, query, data)
}

func (model *SIdentityBaseResource) PostUpdate(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) {
	model.SStandaloneResourceBase.PostUpdate(ctx, userCred, query, data)
}

func (model *SIdentityBaseResource) PostDelete(ctx context.Context, userCred mcclient.TokenCredential) {
	model.SStandaloneResourceBase.PostDelete(ctx, userCred)
}

func (manager *SIdentityBaseResourceManager) totalCount(scope rbacscope.TRbacScope, ownerId mcclient.IIdentityProvider) int {
	q := manager.Query()
	if scope != rbacscope.ScopeSystem {
		q = q.Equals("domain_id", ownerId.GetProjectDomainId())
	}
	cnt, _ := q.CountWithError()
	return cnt
}

func (manager *SIdentityBaseResourceManager) ListItemExportKeys(ctx context.Context, q *sqlchemy.SQuery, userCred mcclient.TokenCredential, keys stringutils2.SSortedStrings) (*sqlchemy.SQuery, error) {
	q, err := manager.SStandaloneResourceBaseManager.ListItemExportKeys(ctx, q, userCred, keys)
	if err != nil {
		return nil, errors.Wrap(err, "SStandaloneResourceBaseManager.ListItemExportKeys")
	}
	q, err = manager.SDomainizedResourceBaseManager.ListItemExportKeys(ctx, q, userCred, keys)
	if err != nil {
		return nil, errors.Wrap(err, "SDomainizedResourceBaseManager.ListItemExportKeys")
	}
	return q, nil
}

func (manager *SIdentityBaseResourceManager) GetPropertyDomainTagValuePairs(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
) (jsonutils.JSONObject, error) {
	return db.GetPropertyTagValuePairs(
		manager.GetIIdentityModelManager(),
		"domain",
		"domain_id",
		ctx,
		userCred,
		query,
	)
}

func (manager *SIdentityBaseResourceManager) GetPropertyDomainTagValueTree(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
) (jsonutils.JSONObject, error) {
	return db.GetPropertyTagValueTree(
		manager.GetIIdentityModelManager(),
		"domain",
		"domain_id",
		ctx,
		userCred,
		query,
	)
}
