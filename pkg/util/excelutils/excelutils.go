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

package excelutils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/360EntSecGroup-Skylar/excelize"

	"yunion.io/x/jsonutils"
	"yunion.io/x/pkg/errors"
)

func decimalBaseMaxWidth(decNum int, base int) int {
	if decNum == 0 {
		return 1
	}
	width := 0
	for decNum > 0 {
		decNum = decNum / base
		width += 1
	}
	return width
}

func decimalBaseN(decNum int, base int, width int) (int, int) {
	b := 1
	for i := 0; i < width-1; i += 1 {
		decNum = decNum / base
		b = b * base
	}
	return decNum, b
}

func decimal2Base(decNum int, base int) []int {
	width := decimalBaseMaxWidth(decNum, base)
	ret := make([]int, width)
	for i := width; i > 0; i -= 1 {
		ith, divider := decimalBaseN(decNum, base, i)
		decNum -= ith * divider
		ret[width-i] = ith
	}
	return ret
}

func decimal2Alphabet(decNum int) string {
	var buf bytes.Buffer
	b26 := decimal2Base(decNum, 26)
	for i := 0; i < len(b26); i += 1 {
		if i == 0 && len(b26) > 1 {
			buf.WriteByte(byte('A' + b26[i] - 1))
		} else {
			buf.WriteByte(byte('A' + b26[i]))
		}
	}
	return buf.String()
}

func exportHeader(xlsx *excelize.File, texts []string, rowIndex int, sheet string) {
	if len(sheet) == 0 {
		sheet = DEFAULT_SHEET
	}
	for i := 0; i < len(texts); i += 1 {
		cell := fmt.Sprintf("%s%d", decimal2Alphabet(i), rowIndex)
		xlsx.SetCellValue(sheet, cell, texts[i])
	}
}

func exportRow(xlsx *excelize.File, data jsonutils.JSONObject, keys []string, rowIndex int, sheet string) {
	if len(sheet) == 0 {
		sheet = DEFAULT_SHEET
	}
	for i := 0; i < len(keys); i += 1 {
		cell := fmt.Sprintf("%s%d", decimal2Alphabet(i), rowIndex)
		// var valStr string
		var val jsonutils.JSONObject
		if strings.Contains(keys[i], ".") {
			val, _ = data.GetIgnoreCases(strings.Split(keys[i], ".")...)
		} else {
			val, _ = data.GetIgnoreCases(keys[i])
		}
		if val != nil {
			// hack, make floating point number prettier
			if fval, ok := val.(*jsonutils.JSONFloat); ok {
				f, _ := fval.Float()
				fvalResult, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", f), 64)
				xlsx.SetCellValue(sheet, cell, fvalResult)
			} else if ival, ok := val.(*jsonutils.JSONInt); ok {
				i, _ := ival.Int()
				xlsx.SetCellValue(sheet, cell, i)
			} else if bval, ok := val.(*jsonutils.JSONBool); ok {
				b, _ := bval.Bool()
				xlsx.SetCellValue(sheet, cell, b)
			} else {
				s, _ := val.GetString()
				xlsx.SetCellValue(sheet, cell, s)
			}
		}
	}
}

func Export(data []jsonutils.JSONObject, keys []string, texts []string, writer io.Writer) error {
	xlsx := excelize.NewFile()
	exportHeader(xlsx, texts, 1, "")
	for i := 0; i < len(data); i += 1 {
		exportRow(xlsx, data[i], keys, i+2, "")
	}
	return xlsx.Write(writer)
}

// key:data中对应的key,text:头
func ExportFile(data []jsonutils.JSONObject, keys []string, texts []string, filename string) error {
	writer, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer writer.Close()
	return Export(data, keys, texts, writer)
}

func ExportWriter(data []jsonutils.JSONObject, keys []string, texts []string, writer io.Writer) *excelize.File {
	xlsx := excelize.NewFile()
	exportHeader(xlsx, texts, 1, "")
	for i := 0; i < len(data); i += 1 {
		exportRow(xlsx, data[i], keys, i+2, "")
	}
	return xlsx
}

type SExcelChartSeries struct {
	Name       string
	Categories string
	Values     string
}

type SExcelChartTitle struct {
	Name string
}

type SExcelChartFormat struct {
	XScale          float64 `json:"x_scale"`
	YScale          float64 `json:"y_scale"`
	XOffset         float64 `json:"x_offset"`
	YOffset         float64 `json:"y_offset"`
	PrintObj        bool    `json:"print_obj"`
	LockAspectRatio bool    `json:"lock_aspect_ratio"`
	Locked          bool    `json:"locked"`
}

type SExcelChartParams struct {
	Type   ExcelChartType
	Series []SExcelChartSeries
	Title  SExcelChartTitle
	Format SExcelChartFormat
}

func AddNewSheet(data []jsonutils.JSONObject, keys []string, texts []string, sheet string, f *excelize.File) *excelize.File {
	// 新建sheet
	f.NewSheet(sheet)
	exportHeader(f, texts, 1, sheet)
	for i := 0; i < len(data); i += 1 {
		exportRow(f, data[i], keys, i+2, sheet)
	}

	return f
}

func AddChartWithSheet(key, sheet string, chartType ExcelChartType, f *excelize.File) (*excelize.File, error) {
	// 获取对应sheet中的所有header
	keys := f.GetRows(sheet)
	if len(keys) == 0 {
		return nil, errors.Errorf("empty sheet")
	}
	// 获取header对应的坐标
	keyIndex := 'A'
	for k, v := range keys[0] {
		if v == key {
			keyIndex = keyIndex + rune(k)
		}
	}
	// 图表请求
	params := SExcelChartParams{
		Type: chartType,
		Series: []SExcelChartSeries{
			{
				Name:       fmt.Sprintf("'%s'!$%s$1", sheet, string(keyIndex)),
				Categories: fmt.Sprintf("'%s'!$A$2:$A$%d", sheet, len(keys)),
				Values:     fmt.Sprintf("'%s'!$%s$2:$%s$%d", sheet, string(keyIndex), string(keyIndex), len(keys))},
		},
		Title: SExcelChartTitle{Name: fmt.Sprintf("%s - %s", key, ChartMap[chartType])},
		Format: SExcelChartFormat{
			XScale:   2.7,
			YScale:   2.9,
			PrintObj: true,
		},
	}
	paramObj := jsonutils.Marshal(params)
	err := f.AddChart(sheet, "A1", paramObj.PrettyString())
	if err != nil {
		return nil, err
	}
	return f, nil
}

func CheckChartType(chartType string) bool {
	_, isExist := ChartMap[ExcelChartType(chartType)]
	return isExist
}
