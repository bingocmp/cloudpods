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

package service

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	apis "yunion.io/x/onecloud/pkg/apis/generalservice"
	"yunion.io/x/onecloud/pkg/appsrv"
	"yunion.io/x/onecloud/pkg/appsrv/dispatcher"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/generalservice/models"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
)

func initHandlers(app *appsrv.Application) {
	db.InitAllManagers()
	taskman.AddTaskHandler("", app)

	db.AddScopeResourceCountHandler("", app)

	for _, manager := range []db.IModelManager{
		taskman.TaskManager,
		taskman.SubTaskManager,
		taskman.TaskObjectManager,
	} {
		db.RegisterModelManager(manager)
	}

	for _, manager := range []db.IModelManager{
		db.Metadata,
		models.GeneralServiceLockManager,
		models.GeneralServiceManager,
		models.GeneralServiceProviderManager,
	} {
		db.RegisterModelManager(manager)
		handler := db.NewModelHandler(manager)
		dispatcher.AddModelDispatcher("", app, handler)
	}
}

func initWebSocketHook() error {
	var services []models.SGeneralService
	err := models.GeneralServiceManager.Query().All(&services)
	if err != nil {
		log.Errorf("fetch general service error %s", err)
		return nil
	}
	for _, service := range services {
		go connectWebSocketHook(service)
	}
	return nil
}

func connectWebSocketHook(service models.SGeneralService) {
	var getProvidersConfig = func(serviceId string) map[string]map[string]interface{} {
		var configs = map[string]map[string]interface{}{}
		providers := models.GeneralServiceProviderManager.GetProviders(serviceId, "")
		for _, provider := range providers {
			config, _ := jsonutils.ParseString(provider.ProviderConfig)
			if config != nil {
				tmp := map[string]interface{}{}
				config.Unmarshal(&tmp)
				configs[provider.Id] = tmp
			}
		}
		return configs
	}

	for {
		service = *models.GeneralServiceManager.FetchServiceById(service.Id)

		serverAddr := strings.ReplaceAll(service.ApiEndpoint, `http://`, "ws://") + "/config"
		conn, _, err := websocket.DefaultDialer.Dial(serverAddr, http.Header{"X-Auth-Token": []string{auth.AdminCredential().GetTokenString()}})
		if err != nil {
			models.GeneralServiceManager.TableSpec().Update(context.Background(), &service, func() error {
				service.Status = apis.SERVICE_UNAVAILABLE
				service.Reason = err.Error()
				return nil
			})
			log.Errorf("Websocket connection %v failed: %v", serverAddr, err)
			time.Sleep(time.Second * 10)

			continue
		}

		err = conn.WriteJSON(getProvidersConfig(service.Id))
		if err != nil {
			log.Errorf("Websocket write %v error: %v", serverAddr, err)
			conn.Close()
			continue
		}

		models.GeneralServiceManager.TableSpec().Update(context.Background(), &service, func() error {
			service.Status = apis.SERVICE_AVAILABLE
			service.Reason = ""
			return nil
		})
		log.Infof("Websocket connection %v established", serverAddr)

		close := make(chan struct{}, 1)
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)

		go func(conn *websocket.Conn) {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					err := conn.WriteMessage(websocket.PingMessage, nil)
					if err != nil {
						return
					}
				}
			}
		}(conn)

		go func() {
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					close <- struct{}{}
					return
				}
			}
		}()

		for {
			select {
			case <-interrupt:
				log.Errorf("Interrupt signal received, closing websocket connection...")
				conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				conn.Close()
				return
			case <-close:
				log.Errorf("Websocket connection %v was closed by remote server", serverAddr)
				conn.Close()
				break
			case service := <-models.UpdatedChan:
				go connectWebSocketHook(*service)
				return
			}
			break
		}

		continue
	}
}
