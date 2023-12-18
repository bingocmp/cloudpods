package tasks

import (
	"context"
	"net/http"

	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/generalservice/models"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
	"yunion.io/x/sqlchemy"
)

type baseTask struct {
	taskman.STask
	TableSpec       *sqlchemy.STableSpec
	Instance        interface{}
	Service         *models.SGeneralService
	ServiceLock     *models.SGeneralServiceLock
	ServiceProvider *models.SGeneralServiceProvider
}

func (its *baseTask) getHeader() http.Header {
	return http.Header{
		"X-Auth-Token":  []string{its.UserCred.GetTokenString()},
		"X-Provider-Id": []string{its.ServiceProvider.Id},
	}
}

func (its *baseTask) OnInit(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) error {
	var err error

	its.ServiceLock = obj.(*models.SGeneralServiceLock)

	if its.ServiceLock.ServiceId != "" {
		is, err := models.GeneralServiceManager.FetchById(its.ServiceLock.ServiceId)
		if err != nil {
			return err
		}
		its.Service = is.(*models.SGeneralService)
	}

	if its.ServiceLock.ProviderId != "" {
		isp, err := models.GeneralServiceProviderManager.FetchById(its.ServiceLock.ProviderId)
		if err != nil {
			return err
		}
		its.ServiceProvider = isp.(*models.SGeneralServiceProvider)
	}

	its.Instance, err = its.Service.NewServiceInstance()
	if err != nil {
		return err
	}
	its.TableSpec = sqlchemy.NewTableSpecFromStruct(its.Instance, stringutils2.Camel2Case(its.Service.Code)+"_tbl")

	if its.ServiceLock.InstanceId != "" {
		err = its.TableSpec.Query().Equals("id", its.ServiceLock.InstanceId).First(its.Instance)
		if err != nil {
			return err
		}
	}

	return nil
}
