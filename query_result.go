package queryrunner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/samber/lo"
	"github.com/zclconf/go-cty/cty"
)

type QueryResult struct {
	Name    string
	Query   string
	Columns []string
	Rows    [][]string
}

func NewEmptyQueryResult(name string, query string) *QueryResult {
	return &QueryResult{
		Name:    name,
		Query:   query,
		Columns: make([]string, 0),
		Rows:    make([][]string, 0),
	}
}

func NewQueryResult(name string, query string, columns []string, rows [][]string) *QueryResult {
	return &QueryResult{
		Name:    name,
		Query:   query,
		Columns: columns,
		Rows:    rows,
	}
}

func NewQueryResultWithJSONLines(name string, query string, lines [][]byte) *QueryResult {
	columnsMap := make(map[string]int)
	rowsMap := make([]map[string]interface{}, 0, len(lines))
	for _, line := range lines {
		var v map[string]interface{}
		log.Println("[debug] NewQueryResultWithJSONLines:", string(line))
		if err := json.Unmarshal(line, &v); err == nil {
			rowsMap = append(rowsMap, v)
			for columnName := range v {
				if _, ok := columnsMap[columnName]; !ok {
					columnsMap[columnName] = len(columnsMap)
				}
			}
		} else {
			log.Println("[warn] unmarshal err", err)
		}
	}
	return NewQueryResultWithRowsMap(name, query, columnsMap, rowsMap)
}

func NewQueryResultWithRowsMap(name, query string, columnsMap map[string]int, rowsMap []map[string]interface{}) *QueryResult {
	queryResults := &QueryResult{
		Name:  name,
		Query: query,
	}
	columnsEntries := lo.Entries(columnsMap)
	sort.Slice(columnsEntries, func(i, j int) bool {
		return columnsEntries[i].Value < columnsEntries[j].Value
	})
	rows := make([][]string, 0, len(rowsMap))
	for _, rowMap := range rowsMap {
		row := make([]string, 0, len(columnsEntries))
		for _, e := range columnsEntries {
			if v, ok := rowMap[e.Key]; ok {
				row = append(row, fmt.Sprintf("%v", v))
			} else {
				row = append(row, "")
			}
		}
		rows = append(rows, row)
	}
	queryResults.Rows = rows
	queryResults.Columns = lo.Map(columnsEntries, func(e lo.Entry[string, int], _ int) string {
		return e.Key
	})
	return queryResults
}

func (qr *QueryResult) ToTable(optFns ...func(*tablewriter.Table)) string {
	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)
	table.SetHeader(qr.Columns)
	for _, optFn := range optFns {
		optFn(table)
	}
	table.AppendBulk(qr.Rows)
	table.Render()
	return buf.String()
}

func (qr *QueryResult) ToVertical() string {
	var builder strings.Builder
	for i, row := range qr.Rows {
		fmt.Fprintf(&builder, "********* %d. row *********\n", i+1)
		for j, column := range qr.Columns {
			fmt.Fprintf(&builder, "  %s: %s\n", column, row[j])
		}
	}
	return builder.String()
}

func (qr *QueryResult) ToBorderlessTable() string {
	return qr.ToTable(
		func(table *tablewriter.Table) {
			table.SetCenterSeparator(" ")
			table.SetAutoFormatHeaders(false)
			table.SetAutoWrapText(false)
			table.SetBorder(false)
			table.SetColumnSeparator(" ")
		},
	)
}

func (qr *QueryResult) ToMarkdownTable() string {
	return qr.ToTable(
		func(table *tablewriter.Table) {
			table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
			table.SetCenterSeparator("|")
			table.SetAutoFormatHeaders(false)
			table.SetAutoWrapText(false)
		},
	)
}

func (qr *QueryResult) ToJSONLines() string {
	var builder strings.Builder
	encoder := json.NewEncoder(&builder)
	for _, row := range qr.toJSON() {
		encoder.Encode(row)
	}
	return builder.String()
}

func (qr *QueryResult) toJSON() []map[string]string {
	columns := make([]string, 0, len(qr.Columns))
	duplicate := make(map[string]int, len(qr.Columns))
	for _, column := range qr.Columns {
		if index, ok := duplicate[column]; ok {
			columns = append(columns, fmt.Sprintf("%s%d", column, index))
			duplicate[column]++
		} else {
			columns = append(columns, column)
			duplicate[column] = 1
		}
	}
	ret := make([]map[string]string, 0, len(qr.Rows))
	for _, row := range qr.Rows {
		v := make(map[string]string, len(qr.Columns))
		for i, column := range columns {
			v[column] = row[i]
		}
		ret = append(ret, v)
	}
	return ret
}

func (qr *QueryResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(qr.toJSON())
}

func (qr *QueryResult) MarshalCTYValue() cty.Value {
	columns := cty.ListValEmpty(cty.String)
	rows := cty.ListValEmpty(cty.List(cty.String))
	if len(qr.Columns) > 0 {
		columns = cty.ListVal(lo.Map(qr.Columns, func(column string, _ int) cty.Value {
			return cty.StringVal(column)
		}))
	}
	if len(qr.Rows) > 0 {
		rows = cty.ListVal(lo.Map(qr.Rows, func(row []string, _ int) cty.Value {
			if len(row) == 0 {
				return cty.ListValEmpty(cty.String)
			}
			return cty.ListVal(lo.Map(row, func(v string, _ int) cty.Value {
				return cty.StringVal(v)
			}))
		}))
	}
	return cty.ObjectVal(map[string]cty.Value{
		"name":             cty.StringVal(qr.Name),
		"query":            cty.StringVal(qr.Query),
		"columns":          columns,
		"rows":             rows,
		"table":            cty.StringVal(qr.ToTable()),
		"markdown_table":   cty.StringVal(qr.ToMarkdownTable()),
		"borderless_table": cty.StringVal(qr.ToBorderlessTable()),
		"vertical_table":   cty.StringVal(qr.ToVertical()),
		"json_lines":       cty.StringVal(qr.ToJSONLines()),
	})
}

type QueryResults []*QueryResult

func (qrs QueryResults) MarshalJSON() ([]byte, error) {
	m := make(map[string]*QueryResult, len(qrs))
	for _, qr := range qrs {
		m[qr.Name] = qr
	}
	return json.Marshal(m)
}
