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
	"fmt"
	"strings"

	json "github.com/json-iterator/go"
	dynamicstruct "github.com/ompluscator/dynamic-struct"
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/generalservice/util"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
)

var fieldType = map[string]interface{}{
	"int":     0,
	"integer": 0,
	"string":  "",
	"float64": 0.0,
	"bool":    false,
}

type ServiceInstanceField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Width       int    `json:"width"`
	Description string `json:"description"`
}

type SServiceInstance struct {
	db.SStandaloneResourceBase
	CloudRegionId     string `width:"128" list:"user" update:"user" json:"cloud_region_id"`
	CloudRegion       string `width:"128" list:"user" update:"user" json:"cloud_region"`
	ProviderId        string `width:"128" list:"user" nullable:"false" create:"required" json:"provider_id"`
	ExternalId        string `width:"256" charset:"utf8" index:"true" list:"user" create:"domain_optional" update:"admin" json:"external_id"`
	Project           string `width:"128" list:"user" json:"project"`
	ProjectId         string `name:"tenant_id" width:"128" charset:"ascii" nullable:"false" index:"true" list:"user" json:"tenant_id"`
	DomainId          string `width:"128" default:"default" nullable:"false" index:"true" list:"user" json:"domain_id"`
	ProjectDomain     string `width:"128" list:"user" json:"project_domain"`
	ReleaseId         string `width:"128" list:"user" json:"release_id"`
	ReleaseStatus     string `width:"128" list:"user" json:"release_status"`
	ReleaseFailReason string `length:"text" list:"user" json:"release_fail_reason"`
	InstanceDetail    string `length:"text" list:"user" json:"instance_detail"`
	RequestParams     string `length:"text" list:"user" json:"request_params"`
	LastOperation     string `width:"128" list:"user" json:"last_operation"`
}

func (its *SGeneralService) getDynamicsBuilder() (dynamicstruct.Builder, error) {
	var fields []*ServiceInstanceField
	jsonObj, err := jsonutils.ParseString(its.DataSchema)
	if err != nil {
		return nil, err
	}
	err = jsonObj.Unmarshal(&fields)
	if err != nil {
		return nil, err
	}
	newStruct := dynamicstruct.ExtendStruct(SServiceInstance{})
	for _, field := range fields {
		if field.Name == "id" || field.Name == "name" {
			continue
		}
		field.Name = strings.Title(field.Name)
		tag := `list:"user" create:"optional" json:"` + stringutils2.Camel2Case(field.Name) + `"`
		if field.Width > 0 {
			tag += fmt.Sprintf(` width:"%d"`, field.Width)
		}
		newStruct.AddField(field.Name, fieldType[field.Type], tag)
	}
	return newStruct, nil
}

func (its *SGeneralService) NewServiceInstance() (interface{}, error) {
	builder, err := its.getDynamicsBuilder()
	if err != nil {
		return nil, err
	}
	return builder.Build().New(), nil
}

func (its *SGeneralService) NewServiceInstances() (interface{}, error) {
	builder, err := its.getDynamicsBuilder()
	if err != nil {
		return nil, err
	}
	return builder.Build().NewSliceOfStructs(), nil
}

func UnmarshalSnakeCase(data []byte, obj interface{}) error {
	var tmpMap map[string]interface{}
	err := json.Unmarshal(data, &tmpMap)
	if err != nil {
		return err
	}
	data, err = json.Marshal(util.JsonSnakeCase{Value: tmpMap})
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, obj)
	return err
}
