package util

import (
	"strings"

	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/cloudcommon/consts"
	"yunion.io/x/pkg/util/printutils"
	"yunion.io/x/sqlchemy"
)

func Paging(q *sqlchemy.SQuery, limit, offset int64, orderBy []jsonutils.JSONObject, order string, obj interface{}) (printutils.ListResult, error) {
	resultList := printutils.ListResult{Data: []jsonutils.JSONObject{}}

	total := q.Count()
	if total == 0 {
		return resultList, nil
	}

	maxLimit := consts.GetMaxPagingLimit()
	if limit == 0 {
		limit = consts.GetDefaultPagingLimit()
	}

	if offset < 0 {
		offset = int64(total) + offset
	}
	if offset < 0 {
		offset = 0
	}
	if int64(total) > maxLimit && (limit <= 0 || limit > maxLimit) {
		limit = maxLimit
	}
	if limit > 0 {
		q = q.Limit(int(limit))
	}
	if offset > 0 {
		q = q.Offset(int(offset))
	}

	if orderBy != nil && order != "" {
		for _, by := range orderBy {
			switch strings.ToLower(order) {
			case "desc":
				q = q.Desc(by.String())
			case "asc":
				q = q.Asc(by.String())
			}
		}
	}

	err := q.All(obj)
	if err != nil {
		return resultList, err
	}

	resultList.Total = total
	resultList.Limit = int(limit)
	resultList.Offset = int(offset)
	resultList.Data, _ = jsonutils.Marshal(obj).GetArray()

	return resultList, nil
}
