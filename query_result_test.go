package queryrunner_test

import (
	"strings"
	"testing"

	"github.com/mashiike/queryrunner"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestNewQueryResultWithJSONLines(t *testing.T) {
	lines := [][]byte{
		[]byte(`{"name":"hoge"}`),
		[]byte(`{"name":"fuga", "age": 18}`),
		[]byte(`{"age": 82, "name":"piyo"}`),
		[]byte(`{"name":"tora", "memo": "animal"}`),
	}
	qr := queryrunner.NewQueryResultWithJSONLines("dummy", "SELECT * FROM dummy", lines)
	expected := &queryrunner.QueryResult{
		Name:    "dummy",
		Query:   "SELECT * FROM dummy",
		Columns: []string{"name", "age", "memo"},
		Rows: [][]string{
			{"hoge", "", ""},
			{"fuga", "18", ""},
			{"piyo", "82", ""},
			{"tora", "", "animal"},
		},
	}
	require.EqualValues(t, expected, qr)
}

func TestQueryReusltMarshalCTYValue(t *testing.T) {
	qr := queryrunner.NewQueryResult(
		"hoge_result",
		"dummy",
		[]string{"Name", "Sign", "Rating"},
		[][]string{
			{"A", "The Good", "500"},
			{"B", "The Very very Bad Man", "288"},
			{"C", "The Ugly", "120"},
			{"D", "The Gopher", "800"},
		},
	)
	value := qr.MarshalCTYValue()
	table := strings.TrimSpace(`
+------+-----------------------+--------+
| NAME |         SIGN          | RATING |
+------+-----------------------+--------+
| A    | The Good              |    500 |
| B    | The Very very Bad Man |    288 |
| C    | The Ugly              |    120 |
| D    | The Gopher            |    800 |
+------+-----------------------+--------+`) + "\n"
	markdownTable := strings.TrimSpace(`
| Name |         Sign          | Rating |
|------|-----------------------|--------|
| A    | The Good              |    500 |
| B    | The Very very Bad Man |    288 |
| C    | The Ugly              |    120 |
| D    | The Gopher            |    800 |`) + "\n"
	borderlessTable := "  " + strings.TrimSpace(`
  Name           Sign            Rating  `+`
------- ----------------------- ---------
  A      The Good                   500  `+`
  B      The Very very Bad Man      288  `+`
  C      The Ugly                   120  `+`
  D      The Gopher                 800`) + "  \n"
	verticalTable := strings.TrimSpace(`
********* 1. row *********
  Name: A
  Sign: The Good
  Rating: 500
********* 2. row *********
  Name: B
  Sign: The Very very Bad Man
  Rating: 288
********* 3. row *********
  Name: C
  Sign: The Ugly
  Rating: 120
********* 4. row *********
  Name: D
  Sign: The Gopher
  Rating: 800`) + "\n"
	jsonLines := strings.TrimSpace(`
{"Name":"A","Rating":"500","Sign":"The Good"}
{"Name":"B","Rating":"288","Sign":"The Very very Bad Man"}
{"Name":"C","Rating":"120","Sign":"The Ugly"}
{"Name":"D","Rating":"800","Sign":"The Gopher"}
`) + "\n"
	require.EqualValues(t, cty.ObjectVal(map[string]cty.Value{
		"name":  cty.StringVal("hoge_result"),
		"query": cty.StringVal("dummy"),
		"columns": cty.ListVal([]cty.Value{
			cty.StringVal("Name"),
			cty.StringVal("Sign"),
			cty.StringVal("Rating"),
		}),
		"rows": cty.ListVal([]cty.Value{
			cty.ListVal([]cty.Value{cty.StringVal("A"), cty.StringVal("The Good"), cty.StringVal("500")}),
			cty.ListVal([]cty.Value{cty.StringVal("B"), cty.StringVal("The Very very Bad Man"), cty.StringVal("288")}),
			cty.ListVal([]cty.Value{cty.StringVal("C"), cty.StringVal("The Ugly"), cty.StringVal("120")}),
			cty.ListVal([]cty.Value{cty.StringVal("D"), cty.StringVal("The Gopher"), cty.StringVal("800")}),
		}),
		"table":            cty.StringVal(table),
		"markdown_table":   cty.StringVal(markdownTable),
		"borderless_table": cty.StringVal(borderlessTable),
		"vertical_table":   cty.StringVal(verticalTable),
		"json_lines":       cty.StringVal(jsonLines),
	}), value)
}

func TestQueryEmptyReusltMarshalCTYValue(t *testing.T) {
	qr := queryrunner.NewEmptyQueryResult("empty", "")
	value := qr.MarshalCTYValue()
	require.EqualValues(t, cty.ObjectVal(map[string]cty.Value{
		"name":             cty.StringVal("empty"),
		"query":            cty.StringVal(""),
		"columns":          cty.ListValEmpty(cty.String),
		"rows":             cty.ListValEmpty(cty.List(cty.String)),
		"table":            cty.StringVal("+\n+\n"),
		"markdown_table":   cty.StringVal(""),
		"borderless_table": cty.StringVal(""),
		"vertical_table":   cty.StringVal(""),
		"json_lines":       cty.StringVal(""),
	}), value)
}
