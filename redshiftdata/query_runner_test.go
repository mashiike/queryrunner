package redshiftdata_test

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/mashiike/hclconfig"
	"github.com/mashiike/queryrunner"
	_ "github.com/mashiike/queryrunner/redshiftdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeBody(t *testing.T) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCLFile("testdata/config.hcl")
	if !assert.False(t, diags.HasErrors()) {
		var builder strings.Builder
		w := hcl.NewDiagnosticTextWriter(&builder, parser.Files(), 400, false)
		w.WriteDiagnostics(diags)
		t.Log(builder.String())
		t.FailNow()
	}
	queries, _, diags := queryrunner.DecodeBody(file.Body, hclconfig.NewEvalContext("./"))
	if !assert.False(t, diags.HasErrors()) {
		var builder strings.Builder
		w := hcl.NewDiagnosticTextWriter(&builder, parser.Files(), 400, false)
		w.WriteDiagnostics(diags)
		t.Log(builder.String())
		t.FailNow()
	}
	require.EqualValues(t, 1, len(queries))

}
